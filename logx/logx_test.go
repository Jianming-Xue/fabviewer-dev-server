package logx

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newBufLogger 返回写入 buffer 的 slog 适配与 buffer 本身，便于断言输出。
func newBufLogger(minLevel slog.Level) (Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	l := NewSlog(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{
		Level: minLevel,
	})))
	return l, buf
}

func TestSlogLevels(t *testing.T) {
	l, buf := newBufLogger(slog.LevelDebug)

	l.Debug("dbg msg", "k", 1)
	l.Info("info msg", "k", 2)
	l.Warn("warn msg", "k", 3)
	l.Error("err msg", "k", 4)

	out := buf.String()
	// TextHandler 会给含空格的 msg 加引号
	assert.Contains(t, out, `level=DEBUG msg="dbg msg"`)
	assert.Contains(t, out, `level=INFO msg="info msg"`)
	assert.Contains(t, out, `level=WARN msg="warn msg"`)
	assert.Contains(t, out, `level=ERROR msg="err msg"`)
}

func TestSlogLevelFiltering(t *testing.T) {
	l, buf := newBufLogger(slog.LevelInfo)

	l.Debug("should be filtered")
	assert.Empty(t, buf.String(), "Debug 低于 handler 级别应被过滤")

	l.Info("should pass")
	assert.Contains(t, buf.String(), "should pass")
}

func TestSlogWith(t *testing.T) {
	l, buf := newBufLogger(slog.LevelDebug)

	derived := l.With("svc", "order", "env", "prod")
	require.NotSame(t, l, derived, "With 应返回派生实例")

	derived.Info("with msg")
	out := buf.String()
	assert.Contains(t, out, "svc=order")
	assert.Contains(t, out, "env=prod")
	assert.Contains(t, out, "with msg")

	t.Run("派生不影响原 logger", func(t *testing.T) {
		l.Info("origin msg")
		for _, line := range strings.Split(buf.String(), "\n") {
			if strings.Contains(line, "origin msg") {
				assert.NotContains(t, line, "svc=order", "原 logger 输出不应带派生字段")
			}
		}
	})
}

func TestNewSlogNil(t *testing.T) {
	orig := slog.Default()
	t.Cleanup(func() { slog.SetDefault(orig) })

	buf := &bytes.Buffer{}
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, nil)))

	l := NewSlog(nil)
	require.NotNil(t, l)
	l.Info("to default")
	assert.Contains(t, buf.String(), "to default")
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

func TestDefaultAndSetDefault(t *testing.T) {
	t.Cleanup(func() { SetDefault(Nop()) })

	t.Run("初始为静默", func(t *testing.T) {
		d := Default()
		require.NotNil(t, d)
		d.Info("silent") // 不 panic 即可
	})

	t.Run("SetDefault 注入后 Default 可见", func(t *testing.T) {
		l, buf := newBufLogger(slog.LevelDebug)
		SetDefault(l)
		Default().Info("via default")
		assert.Contains(t, buf.String(), "via default")
	})

	t.Run("SetDefault(nil) 重置为 Nop", func(t *testing.T) {
		SetDefault(nil)
		d := Default()
		require.NotNil(t, d)
		d.Error("still silent")
	})
}

func TestSetDefaultConcurrent(t *testing.T) {
	t.Cleanup(func() { SetDefault(Nop()) })

	loggers := make([]Logger, 0, 8)
	for i := range 8 {
		loggers = append(loggers, NewSlog(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))).
			With("worker", i))
	}

	var wg sync.WaitGroup
	for i := range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			SetDefault(loggers[i%len(loggers)])
			_ = Default()
			Default().Info("race probe")
		}()
	}
	wg.Wait()
}

func TestInterfaceCompliance(t *testing.T) {
	var _ Logger = (*slogLogger)(nil) // Nop/NewSlog 本身返回 Logger，断言无意义
}
