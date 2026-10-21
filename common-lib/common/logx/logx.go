package logx

import (
	"sync/atomic"

	"go.uber.org/zap"
)

// Logger 是最小日志接口，kv 为 slog 风格 key-value 变长对，不绑定任何日志库。
// 库内组件（中间件、存储包装）与业务经此接缝使用日志；
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

// sugarHolder 同理：持有 Init 落位的 zap 实例，供包级函数直调（caller
// 跳过包装层后指向业务调用行）；SetLogger 注入自定义实现时为 nil，
// 包级函数降级走 Default() 接口。
type sugarHolder struct{ s *zap.SugaredLogger }

var (
	defaultLogger atomic.Value // 存储 defaultHolder；初始为 Nop
	defaultSugar  atomic.Value // 存储 sugarHolder；初始为空（nil）
)

func init() {
	defaultLogger.Store(defaultHolder{l: nopLogger{}})
	defaultSugar.Store(sugarHolder{})
}

// Default 返回库内组件与包级函数共用的全局日志出口，初始为 Nop（静默），
// Init 成功后自动落位，测试可经 SetLogger 注入。
func Default() Logger {
	return defaultLogger.Load().(defaultHolder).l
}

// sharedSugar 返回包级函数直调的 zap 实例；未落位（或已注入自定义
// Logger）时返回 nil。
func sharedSugar() *zap.SugaredLogger {
	return defaultSugar.Load().(sugarHolder).s
}

// SetLogger 设置全局日志出口（Init 内部装配完成后同样经它落位），
// 并发安全；l 为 nil 时重置为 Nop。注入自定义实现时清空包级函数直调
// 通道，包级函数降级经接口转发。
func SetLogger(l Logger) {
	if l == nil {
		l = Nop()
	}
	defaultLogger.Store(defaultHolder{l: l})
	defaultSugar.Store(sugarHolder{})
}

// Nop 返回丢弃全部输出的 Logger。
func Nop() Logger { return nopLogger{} }

type nopLogger struct{}

func (nopLogger) Debug(string, ...any) {}
func (nopLogger) Info(string, ...any)  {}
func (nopLogger) Warn(string, ...any)  {}
func (nopLogger) Error(string, ...any) {}
func (nopLogger) With(...any) Logger   { return nopLogger{} }
