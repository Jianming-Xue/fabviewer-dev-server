package logx

import (
	"log/slog"
	"sync/atomic"
)

// Logger 是最小日志接口，kv 为 slog 风格 key-value 变长对，不绑定任何日志库。
// 业务系统已有 zap/logrus 等设施时实现本接口（五个方法）即可接入；
// 错误对象作为 kv 传入：
//
//	l.Error("query failed", "err", err)
type Logger interface {
	Debug(msg string, kv ...any)
	Info(msg string, kv ...any)
	Warn(msg string, kv ...any)
	Error(msg string, kv ...any)
	// With 返回携带固定字段的派生 Logger。
	With(kv ...any) Logger
}

// defaultHolder 保证 atomic.Value 始终存同一个具体类型（否则 Store 会 panic）。
type defaultHolder struct{ l Logger }

var defaultLogger atomic.Value // 存储 defaultHolder；初始为 Nop

func init() {
	defaultLogger.Store(defaultHolder{l: nopLogger{}})
}

// Default 返回库内组件的兜底日志出口，初始为 Nop（静默），
// 业务系统启动时经 SetDefault 注入。
func Default() Logger {
	return defaultLogger.Load().(defaultHolder).l
}

// SetDefault 设置兜底日志出口，并发安全；l 为 nil 时重置为 Nop。
func SetDefault(l Logger) {
	if l == nil {
		l = Nop()
	}
	defaultLogger.Store(defaultHolder{l: l})
}

// Nop 返回丢弃全部输出的 Logger。
func Nop() Logger { return nopLogger{} }

type nopLogger struct{}

func (nopLogger) Debug(string, ...any) {}
func (nopLogger) Info(string, ...any)  {}
func (nopLogger) Warn(string, ...any)  {}
func (nopLogger) Error(string, ...any) {}
func (nopLogger) With(...any) Logger   { return nopLogger{} }

// slogLogger 将调用桥接到 *slog.Logger。
type slogLogger struct {
	l *slog.Logger
}

// NewSlog 返回基于 slog 的 Logger 实现；l 为 nil 时使用 slog.Default()。
func NewSlog(l *slog.Logger) Logger {
	if l == nil {
		l = slog.Default()
	}
	return &slogLogger{l: l}
}

func (s *slogLogger) Debug(msg string, kv ...any) { s.l.Debug(msg, kv...) }
func (s *slogLogger) Info(msg string, kv ...any)  { s.l.Info(msg, kv...) }
func (s *slogLogger) Warn(msg string, kv ...any)  { s.l.Warn(msg, kv...) }
func (s *slogLogger) Error(msg string, kv ...any) { s.l.Error(msg, kv...) }
func (s *slogLogger) With(kv ...any) Logger       { return &slogLogger{l: s.l.With(kv...)} }
