package logx

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config 配置 New 构建的完整日志工具：引擎为 zap（业界事实标准），
// 出口可组合（至少启用一个）——标准输出 / 文件（编号滚动 log、log.1、
// log.2 ... 保留 N 份）/ 远程出口（异步批量，如 OpenSearch，见
// storage/opensearch/logsink）。每个出口可注入各自的自定义编码器
// （Encoder 字段 + logx.LineEncoder 适配业务格式函数），实现"文件是
// 字符串格式、远程是另一种 JSON"这类组合。
type Config struct {
	Level  string // debug|info|warn|error，默认 info
	Format string // json（默认）|console，未被出口级 Encoder 覆盖时生效
	Stdout bool   // 输出到标准输出
	File   *FileConfig
	Remote Sink

	// StdoutEncoder / RemoteEncoder 覆盖对应出口的默认编码（File 的见
	// FileConfig.Encoder）；典型注入 logx.LineEncoder(业务格式函数)。
	StdoutEncoder zapcore.Encoder
	RemoteEncoder zapcore.Encoder

	// 远程出口调参（均可省略取默认）
	RemoteBatchSize  int           // 凑批行数，默认 256
	RemoteFlushEvery time.Duration // 定时刷出间隔，默认 5s
	RemoteBufferSize int           // 异步缓冲行数，默认 4096；满则丢弃计数（不阻塞业务）
}

// FileConfig 是文件出口配置：编号滚动——写满 MaxSizeMB 时最老留档删除、
// 逐个 +1（log.N → log.N+1）、当前文件改名 log.1、开新文件继续写；
// 命名即 log、log.1、log.2 ... log.MaxBackups。
type FileConfig struct {
	Path       string          // 必填（目录不存在自动创建）
	MaxSizeMB  int             // 单文件上限 MB，默认 100
	MaxBackups int             // 留档个数（log.1 到 log.N），默认 7
	MaxAgeDays int             // 兼容字段，暂未生效（编号滚动按个数保留）
	Compress   bool            // 兼容字段，暂未生效
	Encoder    zapcore.Encoder // 覆盖本出口编码（如 logx.LineEncoder）
}

// Sink 是远程日志出口的契约：Flush 接收一批已编码的日志行（JSON 行），
// 实现方负责投递——如 OpenSearch 批量索引（storage/opensearch/logsink）。
// Flush 由 logx 的异步缓冲器单线程调用，实现无需并发安全；
// 返回 error 时本批丢弃并计数（日志链路永不反压业务）。
type Sink interface {
	Flush(lines [][]byte) error
}

// New 构建完整日志工具：zap 引擎 + 可组合出口，返回 Logger、收尾函数
// （排空远程缓冲并关闭文件，服务停机时调用）与配置错误。
// 业务入口典型用法：
//
//	log, closeLog, err := logx.New(logx.Config{
//	    Stdout: true,
//	    File:   &logx.FileConfig{Path: "app.log"},
//	    Remote: osLogSink, // logsink.New(cli, "logs")，可省
//	})
//	if err != nil { ... }
//	defer closeLog() // 优雅停机处调用
func New(cfg Config) (Logger, func(), error) { //nolint:gocritic //hugeParam：配置值对象按值传入为一次性人体工学约定（同 storage Open 系列）
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, nil, err
	}

	var cores []zapcore.Core
	var closers []func()
	if cfg.Stdout {
		cores = append(cores, zapcore.NewCore(encOr(cfg.StdoutEncoder, cfg.Format),
			zapcore.Lock(os.Stdout), level))
	}
	if cfg.File != nil {
		if cfg.File.Path == "" {
			return nil, nil, errors.New("logx: File.Path 不能为空")
		}
		fw, err := newNumberedWriter(cfg.File.Path,
			int64(orDefault(cfg.File.MaxSizeMB, 100))*1024*1024,
			orDefault(cfg.File.MaxBackups, 7))
		if err != nil {
			return nil, nil, err
		}
		cores = append(cores, zapcore.NewCore(encOr(cfg.File.Encoder, cfg.Format),
			zapcore.AddSync(fw), level))
		closers = append(closers, func() { _ = fw.Close() })
	}
	if cfg.Remote != nil {
		rc, closeFn := newRemoteCore(&cfg, level, encOr(cfg.RemoteEncoder, cfg.Format))
		cores = append(cores, rc)
		closers = append(closers, closeFn)
	}
	if len(cores) == 0 {
		return nil, nil, errors.New("logx: 至少启用一个出口（Stdout/File/Remote）")
	}

	sugar := zap.New(zapcore.NewTee(cores...)).Sugar()
	return zapLogger{s: sugar}, func() {
		for i := len(closers) - 1; i >= 0; i-- {
			closers[i]()
		}
	}, nil
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

// encOr 出口级编码器优先，缺省按 Format 构建。
func encOr(enc zapcore.Encoder, format string) zapcore.Encoder {
	if enc != nil {
		return enc
	}
	return newEncoder(format)
}

// newEncoder 按格式构建编码器（每个出口独立实例，时间 ISO8601、级别小写）。
func newEncoder(format string) zapcore.Encoder {
	ec := zap.NewProductionEncoderConfig()
	ec.EncodeTime = zapcore.ISO8601TimeEncoder
	ec.EncodeLevel = zapcore.LowercaseLevelEncoder
	if strings.EqualFold(format, "console") {
		return zapcore.NewConsoleEncoder(ec)
	}
	return zapcore.NewJSONEncoder(ec)
}

// parseLevel 解析级别字符串，空串取 info。
func parseLevel(v string) (zapcore.Level, error) {
	if v == "" {
		return zapcore.InfoLevel, nil
	}
	l, err := zapcore.ParseLevel(strings.ToLower(v))
	if err != nil {
		return 0, fmt.Errorf("logx: 非法级别 %q（debug|info|warn|error）: %w", v, err)
	}
	return l, nil
}

func orDefault(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

// remoteCore 把 zap 桥接到异步批量缓冲：Write 永不阻塞（缓冲满即丢弃并
// 计数，绝不反压业务），后台协程凑批/定时经 Sink 投递，close 排空收尾。
type remoteCore struct {
	level  zapcore.LevelEnabler
	enc    zapcore.Encoder
	fields []zapcore.Field // With 派生携带的字段（写入时合并编码）
	*remoteShared
}

// remoteShared 是所有 With 派生核心共享的缓冲与投递状态。
type remoteShared struct {
	ch      chan []byte
	sink    Sink
	batch   int
	every   time.Duration
	dropped atomic.Int64
	done    chan struct{}
	wg      sync.WaitGroup
}

func newRemoteCore(cfg *Config, level zapcore.LevelEnabler, enc zapcore.Encoder) (core zapcore.Core, closeFn func()) {
	shared := &remoteShared{
		ch:    make(chan []byte, orDefault(cfg.RemoteBufferSize, 4096)),
		sink:  cfg.Remote,
		batch: orDefault(cfg.RemoteBatchSize, 256),
		every: cfg.RemoteFlushEvery,
		done:  make(chan struct{}),
	}
	if shared.every <= 0 {
		shared.every = 5 * time.Second
	}
	shared.wg.Add(1)
	go shared.loop()
	return &remoteCore{level: level, enc: enc, remoteShared: shared}, shared.close
}

func (c *remoteCore) Enabled(l zapcore.Level) bool { return c.level.Enabled(l) }

func (c *remoteCore) With(fields []zapcore.Field) zapcore.Core {
	own := make([]zapcore.Field, 0, len(c.fields)+len(fields))
	own = append(own, c.fields...)
	own = append(own, fields...)
	return &remoteCore{level: c.level, enc: c.enc.Clone(), fields: own,
		remoteShared: c.remoteShared}
}

func (c *remoteCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry { //nolint:gocritic //hugeParam：zapcore.Core 接口约定按值（与 zap 内置实现一致）
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

func (c *remoteCore) Write(ent zapcore.Entry, fields []zapcore.Field) error { //nolint:gocritic //hugeParam：zapcore.Core 接口约定按值（与 zap 内置实现一致）
	all := make([]zapcore.Field, 0, len(c.fields)+len(fields))
	all = append(all, c.fields...)
	all = append(all, fields...)
	buf, err := c.enc.EncodeEntry(ent, all)
	if err != nil {
		return err
	}
	line := append([]byte(nil), buf.Bytes()...) // 复制出池
	buf.Free()
	select {
	case c.ch <- line:
	default:
		c.dropped.Add(1) // 缓冲满：丢弃保业务，绝不阻塞
	}
	return nil
}

func (c *remoteCore) Sync() error { return nil } // 批量语义由 close 排空承担

// loop 后台投递：凑满 batch 或 every 到期即 Flush；done 后排空剩余退出。
func (s *remoteShared) loop() {
	defer s.wg.Done()
	var pending [][]byte
	ticker := time.NewTicker(s.every)
	defer ticker.Stop()
	for {
		select {
		case line := <-s.ch:
			pending = append(pending, line)
			if len(pending) >= s.batch {
				s.flush(&pending)
			}
		case <-ticker.C:
			s.flush(&pending)
		case <-s.done:
			for {
				select {
				case line := <-s.ch:
					pending = append(pending, line)
				default:
					s.flush(&pending)
					return
				}
			}
		}
	}
}

// flush 投递一批（空批跳过）；失败丢弃本批并计数。
func (s *remoteShared) flush(pending *[][]byte) {
	if len(*pending) == 0 {
		return
	}
	lines := *pending
	*pending = nil
	if err := s.sink.Flush(lines); err != nil {
		s.dropped.Add(int64(len(lines)))
		fmt.Fprintf(os.Stderr, "logx: 远程出口投递失败，丢弃 %d 行: %v\n", len(lines), err)
	}
}

// close 停止后台协程（触发剩余排空）并汇报累计丢弃数。
func (s *remoteShared) close() {
	close(s.done)
	s.wg.Wait()
	if n := s.dropped.Load(); n > 0 {
		fmt.Fprintf(os.Stderr, "logx: 远程出口累计丢弃 %d 行日志\n", n)
	}
}
