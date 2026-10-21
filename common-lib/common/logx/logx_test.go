package logx

import (
	"bytes"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bufLogger 是捕获式 Logger 假实现：按最低等级过滤、把调用写成
// "level msg k=v" 行——用于断言全局出口与包级函数的转发行为。
type bufLogger struct {
	mu     sync.Mutex
	buf    *bytes.Buffer
	min    int // 0=debug 1=info 2=warn 3=error
	fields []any
}

func newBufLogger(minLevel int) *bufLogger {
	return &bufLogger{buf: &bytes.Buffer{}, min: minLevel}
}

func (b *bufLogger) write(lv int, name, msg string, kv ...any) {
	if lv < b.min {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	fmt.Fprintf(b.buf, "%s %s", name, msg)
	for i := 0; i+1 < len(b.fields); i += 2 {
		fmt.Fprintf(b.buf, " %v=%v", b.fields[i], b.fields[i+1])
	}
	for i := 0; i+1 < len(kv); i += 2 {
		fmt.Fprintf(b.buf, " %v=%v", kv[i], kv[i+1])
	}
	b.buf.WriteByte('\n')
}

func (b *bufLogger) Debug(msg string, kv ...any) { b.write(0, "debug", msg, kv...) }
func (b *bufLogger) Info(msg string, kv ...any)  { b.write(1, "info", msg, kv...) }
func (b *bufLogger) Warn(msg string, kv ...any)  { b.write(2, "warn", msg, kv...) }
func (b *bufLogger) Error(msg string, kv ...any) { b.write(3, "error", msg, kv...) }

func (b *bufLogger) With(kv ...any) Logger {
	d := &bufLogger{buf: b.buf, min: b.min}
	d.fields = append(append([]any{}, b.fields...), kv...)
	return d
}

func (b *bufLogger) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestBufLoggerFilterAndWith(t *testing.T) {
	l := newBufLogger(1) // info 及以上

	l.Debug("filtered")
	assert.Empty(t, l.String(), "低于最低等级应被过滤")

	l.Info("pass", "k", 1)
	assert.Contains(t, l.String(), "info pass k=1")

	derived := l.With("svc", "order")
	derived.Warn("derived msg")
	assert.Contains(t, l.String(), "warn derived msg svc=order", "With 字段随派生调用输出")

	l.Info("origin")
	assert.NotContains(t, l.String(), "origin msg svc", "派生不影响原 logger")
	assert.Contains(t, l.String(), "info origin")
}

func TestNop(t *testing.T) {
	l := Nop()
	require.NotNil(t, l)

	// 全部方法可安全调用且无输出（Nop 无 buffer 可查，以不 panic 为准）
	l.Debug("x")
	l.Info("x")
	l.Warn("x")
	l.Error("x")

	derived := l.With("k", "v")
	require.NotNil(t, derived)
	derived.Info("x")
}

func TestDefaultAndSetLogger(t *testing.T) {
	t.Cleanup(func() { SetLogger(Nop()) })

	t.Run("初始为静默", func(t *testing.T) {
		d := Default()
		require.NotNil(t, d)
		d.Info("silent") // 不 panic 即可
	})

	t.Run("SetLogger 注入后 Default 可见", func(t *testing.T) {
		l := newBufLogger(0)
		SetLogger(l)
		Default().Info("via default")
		assert.Contains(t, l.String(), "via default")
	})

	t.Run("SetLogger(nil) 重置为 Nop", func(t *testing.T) {
		SetLogger(nil)
		d := Default()
		require.NotNil(t, d)
		d.Error("still silent")
	})
}

func TestPackageLevelFuncs(t *testing.T) {
	t.Cleanup(func() { SetLogger(Nop()) })

	l := newBufLogger(0)
	SetLogger(l)

	Debug("dbg msg")
	Info("info msg")
	Warn("warn msg")
	Error("err msg", "k", 1)

	out := l.String()
	assert.Contains(t, out, "debug dbg msg")
	assert.Contains(t, out, "info info msg")
	assert.Contains(t, out, "warn warn msg")
	assert.Contains(t, out, "error err msg k=1")

	t.Run("未初始化时静默（Nop）", func(t *testing.T) {
		SetLogger(nil)
		Info("dropped") // 不 panic 即可
	})

	t.Run("注入自定义 Logger 后包级函数降级走接口", func(t *testing.T) {
		l := newBufLogger(0)
		SetLogger(l)
		Debug("via interface dbg")
		Info("via interface", "k", 1)
		Warn("via interface warn")
		Error("via interface error")
		assert.Contains(t, l.String(), "debug via interface dbg")
		assert.Contains(t, l.String(), "info via interface k=1")
		assert.Contains(t, l.String(), "warn via interface warn")
		assert.Contains(t, l.String(), "error via interface error")
	})
}

func TestSetLoggerConcurrent(t *testing.T) {
	t.Cleanup(func() { SetLogger(Nop()) })

	loggers := make([]Logger, 0, 8)
	for i := range 8 {
		loggers = append(loggers, newBufLogger(0).With("worker", i))
	}

	var wg sync.WaitGroup
	for i := range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			SetLogger(loggers[i%len(loggers)])
			_ = Default()
			Default().Info("race probe")
		}()
	}
	wg.Wait()
}

func TestInterfaceCompliance(t *testing.T) {
	var _ Logger = (*bufLogger)(nil)
	var _ Logger = nopLogger{}
}
