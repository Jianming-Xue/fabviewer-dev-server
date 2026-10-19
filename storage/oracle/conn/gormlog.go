package conn

import (
	"context"
	"errors"
	"fmt"
	"time"

	gormlogger "gorm.io/gorm/logger"

	"github.com/fab-viewer/common-lib/common/logx"
	"github.com/fab-viewer/common-lib/storage/oracle/config"
)

// gormLogger 把 gorm/logger.Interface 的五个回调适配到 logx.Logger，
// 并按 Config.LogLevel 过滤：silent 全静默、error 仅错误 SQL、
// warn 增加慢 SQL（默认）、info 输出全部 SQL。
//
// 按计划决策固定 IgnoreRecordNotFoundError=true：gorm.ErrRecordNotFound
// 不产生错误日志（404 语义已在 api/middleware.ErrorHandler 层处理）。
type gormLogger struct {
	log           logx.Logger
	level         gormlogger.LogLevel
	slowThreshold time.Duration
}

// newGormLogger 构造适配器。log 为 nil 时回退 logx.Default()；
// slow <=0 时不判定慢 SQL。
func newGormLogger(log logx.Logger, level config.LogLevel, slow time.Duration) *gormLogger {
	if log == nil {
		log = logx.Default()
	}
	return &gormLogger{log: log, level: logLevelToGorm(level), slowThreshold: slow}
}

// LogMode 实现 gorm/logger.Interface：返回调整级别后的副本，不改动原实例
// （gorm 的 Debug() 会以返回值派生会话级 logger）。
func (l *gormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	copied := *l
	copied.level = level
	return &copied
}

// Info 实现 gorm/logger.Interface：按 info 级别输出 GORM 框架消息。
func (l *gormLogger) Info(_ context.Context, msg string, data ...any) {
	if l.level >= gormlogger.Info {
		l.log.Info(fmt.Sprintf(msg, data...))
	}
}

// Warn 实现 gorm/logger.Interface：按 warn 级别输出 GORM 框架消息。
func (l *gormLogger) Warn(_ context.Context, msg string, data ...any) {
	if l.level >= gormlogger.Warn {
		l.log.Warn(fmt.Sprintf(msg, data...))
	}
}

// Error 实现 gorm/logger.Interface：按 error 级别输出 GORM 框架消息。
func (l *gormLogger) Error(_ context.Context, msg string, data ...any) {
	if l.level >= gormlogger.Error {
		l.log.Error(fmt.Sprintf(msg, data...))
	}
}

// Trace 实现 gorm/logger.Interface：按级别分流 SQL 追踪日志——
// 错误 SQL（跳过 ErrRecordNotFound）> 慢 SQL > 全量 SQL。
func (l *gormLogger) Trace(_ context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.level <= gormlogger.Silent {
		return
	}
	elapsed := time.Since(begin)
	switch {
	case err != nil && l.level >= gormlogger.Error && !errors.Is(err, gormlogger.ErrRecordNotFound):
		sql, rows := fc()
		l.log.Error("gorm sql error", traceKV(elapsed, rows, "sql", sql, "err", err)...)
	case l.isSlow(elapsed) && l.level >= gormlogger.Warn:
		sql, rows := fc()
		l.log.Warn("gorm slow sql", traceKV(elapsed, rows,
			"sql", sql, "slow_threshold_ms", l.slowThreshold.Milliseconds())...)
	case l.level == gormlogger.Info:
		sql, rows := fc()
		l.log.Info("gorm sql", traceKV(elapsed, rows, "sql", sql)...)
	}
}

// isSlow 判定是否超过慢 SQL 阈值；阈值未配置（<=0）时恒为 false。
func (l *gormLogger) isSlow(elapsed time.Duration) bool {
	return l.slowThreshold > 0 && elapsed > l.slowThreshold
}

// traceKV 组装 Trace 日志的公共 key-value：耗时毫秒与影响行数（未知为 -1）。
func traceKV(elapsed time.Duration, rows int64, kv ...any) []any {
	out := make([]any, 0, len(kv)+4)
	out = append(out, "elapsed_ms", float64(elapsed.Nanoseconds())/1e6, "rows", rows)
	return append(out, kv...)
}

// logLevelToGorm 将字符串日志级别映射为 gorm/logger 级别；空值与未知值按 warn。
func logLevelToGorm(l config.LogLevel) gormlogger.LogLevel {
	switch l {
	case config.LogLevelSilent:
		return gormlogger.Silent
	case config.LogLevelError:
		return gormlogger.Error
	case config.LogLevelInfo:
		return gormlogger.Info
	default:
		return gormlogger.Warn
	}
}
