package logx

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/fab-viewer/common-lib/common/config"
)

// ErrNoLogComponent 表示 console 与 file 两个出口均未启用（enabled 都为
// false，或配置中缺失整个 log 段）。
var ErrNoLogComponent = errors.New("logx: 未启用任何日志出口（console/file 的 enabled 均为 false）")

// log 段默认值（2026-10-21 拍板）。
const (
	defaultLogPath    = "systemlogs/server.log"
	defaultMaxSizeMB  = 100 // 单文件上限（兆）
	defaultMaxBackups = 100 // 备份保留份数
	defaultMaxAgeDays = 30  // 备份保留天数
)

// logConfig 是 log 段的私有配置结构（不对外暴露，字段名与 yml key 一致，
// 忽略大小写匹配）。两个子段均可整体缺失（指针为 nil 即未启用）。
type logConfig struct {
	Level   string
	Console *consoleConfig
	File    *fileConfig
}

// consoleConfig 是控制台出口配置。
type consoleConfig struct {
	Enabled bool
	Level   string
	Format  string // json（默认）|console
}

// fileConfig 是文件出口配置，滚动由 lumberjack 承担。
type fileConfig struct {
	Enabled    bool
	Level      string
	Format     string // json（默认）|console
	Path       string
	MaxSize    int // 单文件上限（兆）
	MaxBackups int // 备份保留份数
	MaxAge     int // 备份保留天数
	Compress   bool
	// LocalTime 用指针表达三态：nil（未配置）按默认 true 处理，显式
	// false 关闭本地时间（备份文件名用 UTC）。
	LocalTime *bool
}

// Init 从配置 map 初始化日志组件（组件自治入口，供 cmd/server 统一装配
// 调用）：读取 log 段 → 校验存在的键 → 绑定私有配置并补默认值 → 按
// enabled 构建 console/file 两个 Core 合并。成功后经 SetLogger 落位全局
// 出口，包级函数 Info/Warn/Error/Debug 随即生效；返回的收尾函数负责
// Sync 与关闭文件（优雅停机时调用）。
func Init(m map[string]any) (Logger, func(), error) {
	cfg, err := loadLogConfig(m)
	if err != nil {
		return nil, nil, err
	}

	var cores []zapcore.Core
	var closers []func()
	if cfg.Console != nil && cfg.Console.Enabled {
		cores = append(cores, zapcore.NewCore(
			newEncoder(cfg.Console.Format),
			zapcore.Lock(os.Stdout),
			resolveLevel(cfg.Console.Level, cfg.Level)))
	}
	if cfg.File != nil && cfg.File.Enabled {
		fc := withFileDefaults(cfg.File)
		lj := &lumberjack.Logger{
			Filename:   fc.Path,
			MaxSize:    fc.MaxSize,
			MaxBackups: fc.MaxBackups,
			MaxAge:     fc.MaxAge,
			Compress:   fc.Compress,
			LocalTime:  fc.LocalTime == nil || *fc.LocalTime,
		}
		cores = append(cores, zapcore.NewCore(
			newEncoder(fc.Format),
			zapcore.AddSync(lj),
			resolveLevel(fc.Level, cfg.Level)))
		closers = append(closers, func() { _ = lj.Close() })
	}
	if len(cores) == 0 {
		return nil, nil, ErrNoLogComponent
	}

	// AddCallerSkip(1)：caller 跳过 zapLogger 包装层，日志的调用位置
	// 指向真实的业务/组件调用行（包级函数另经 sharedSugar 直调 zap，
	// 见 log.go）
	sugar := zap.New(zapcore.NewTee(cores...), zap.AddCaller(), zap.AddCallerSkip(1)).Sugar()
	logger := zapLogger{s: sugar}
	cleanup := func() {
		_ = sugar.Sync()
		for i := len(closers) - 1; i >= 0; i-- {
			closers[i]()
		}
	}
	SetLogger(logger)                         // 落位接口出口（会清空直调通道）
	defaultSugar.Store(sugarHolder{s: sugar}) // 再落位包级函数直调通道
	return logger, cleanup, nil
}

// zapLogger 把 Logger 接口桥接到 zap 的结构化 kv 调用。
type zapLogger struct {
	s *zap.SugaredLogger
}

func (l zapLogger) Debug(msg string, kv ...any) { l.s.Debugw(msg, kv...) }
func (l zapLogger) Info(msg string, kv ...any)  { l.s.Infow(msg, kv...) }
func (l zapLogger) Warn(msg string, kv ...any)  { l.s.Warnw(msg, kv...) }
func (l zapLogger) Error(msg string, kv ...any) { l.s.Errorw(msg, kv...) }
func (l zapLogger) With(kv ...any) Logger       { return zapLogger{s: l.s.With(kv...)} }

// loadLogConfig 读取并校验 log 段：先对存在的键做断言式校验（enabled 用
// ValidateIsBoolean，maxSize/maxBackups/maxAge 用 ValidateIsNumber，环境
// 变量覆盖产生的字符串按宽松规则放行），再绑定到私有配置；log 段整体
// 缺失时返回零值配置（后续按未启用任何出口处理）。
func loadLogConfig(m map[string]any) (*logConfig, error) {
	sub := config.Get(m, "log")
	if sub == nil {
		return &logConfig{}, nil // 无 log 段：零值配置（未启用任何出口）
	}
	checks := []struct {
		seg, key string
		fn       func(map[string]any, string) error
	}{
		{"console", "enabled", config.ValidateIsBoolean},
		{"file", "enabled", config.ValidateIsBoolean},
		{"file", "maxSize", config.ValidateIsNumber},
		{"file", "maxBackups", config.ValidateIsNumber},
		{"file", "maxAge", config.ValidateIsNumber},
	}
	for _, c := range checks {
		if err := validateIfPresent(sub, c.seg, c.key, c.fn); err != nil {
			return nil, err
		}
	}
	cfg, err := config.GetSection[logConfig](m, "log")
	if err != nil {
		return nil, fmt.Errorf("logx: %w", err)
	}
	return cfg, nil
}

// validateIfPresent 对 log 段下 "seg.key" 的键做"存在才校验"：键缺失（将取
// 默认值）不校验，存在则经 fn 断言。
func validateIfPresent(log map[string]any, seg, key string, fn func(map[string]any, string) error) error {
	child := config.Get(log, seg)
	if child == nil {
		return nil
	}
	if _, ok := child[key]; !ok {
		return nil
	}
	if err := fn(log, seg+"."+key); err != nil {
		return fmt.Errorf("logx: log.%s.%s %w", seg, key, err)
	}
	return nil
}

// withFileDefaults 补齐文件出口默认值：path/maxSize/maxBackups/maxAge；
// Compress 默认 false、LocalTime 默认 true 已在构造处处理。
func withFileDefaults(fc *fileConfig) *fileConfig {
	if fc.Path == "" {
		fc.Path = defaultLogPath
	}
	fc.MaxSize = orDefault(fc.MaxSize, defaultMaxSizeMB)
	fc.MaxBackups = orDefault(fc.MaxBackups, defaultMaxBackups)
	fc.MaxAge = orDefault(fc.MaxAge, defaultMaxAgeDays)
	return fc
}

// resolveLevel 解析出口等级：优先出口自身 level，为空或非法取值退全局
// level，仍无效兜底 info（静默处理，不报错）。
func resolveLevel(outlet, global string) zapcore.Level {
	for _, v := range []string{outlet, global} {
		if l, ok := parseLevel(v); ok {
			return l
		}
	}
	return zapcore.InfoLevel
}

// parseLevel 解析四档等级字符串（debug/info/warn/error），不认识返回
// false。
func parseLevel(v string) (zapcore.Level, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "debug":
		return zapcore.DebugLevel, true
	case "info":
		return zapcore.InfoLevel, true
	case "warn":
		return zapcore.WarnLevel, true
	case "error":
		return zapcore.ErrorLevel, true
	}
	return 0, false
}

// newEncoder 按出口 format 构建编码器：console 取 zap 原生 console
// encoder，其余（json、空串与非法取值）一律 json——与等级的静默兜底
// 一致。每个出口独立实例（时间 ISO8601、级别小写）。
func newEncoder(format string) zapcore.Encoder {
	ec := zap.NewProductionEncoderConfig()
	ec.EncodeTime = zapcore.ISO8601TimeEncoder
	ec.EncodeLevel = zapcore.LowercaseLevelEncoder
	if strings.EqualFold(strings.TrimSpace(format), "console") {
		return zapcore.NewConsoleEncoder(ec)
	}
	return zapcore.NewJSONEncoder(ec)
}

func orDefault(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}
