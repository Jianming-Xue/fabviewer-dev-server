package logx

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

// errEncoder 编码必失败（覆盖 Write 错误分支）。
type errEncoder struct{ zapcore.Encoder }

func (errEncoder) EncodeEntry(zapcore.Entry, []zapcore.Field) (*buffer.Buffer, error) {
	return nil, errors.New("encode boom")
}

func TestRemoteCoreInternals(t *testing.T) {
	sink := &internalSink{}
	core, closeFn := newRemoteCore(&Config{
		Remote: sink, RemoteBatchSize: 1, RemoteFlushEvery: time.Hour,
	}, zapcore.InfoLevel, newEncoder("json"))
	t.Cleanup(closeFn)

	// With 派生：字段随行编码（经内部字段合并路径）
	derived := core.With([]zapcore.Field{zap.String("svc", "demo")})
	require.NoError(t, derived.(*remoteCore).Write(
		zapcore.Entry{Level: zapcore.InfoLevel, Time: time.Now()}, nil))
	require.Eventually(t, func() bool { return sink.got() == 1 },
		2*time.Second, 5*time.Millisecond)
	line := sink.last()
	assert.Contains(t, line, `"svc":"demo"`)

	// 级别不足：Check 不登记
	ce := derived.Check(
		zapcore.Entry{Level: zapcore.DebugLevel, Time: time.Now()}, nil)
	assert.Nil(t, ce, "debug 低于 info 不登记")

	// Sync 空实现
	require.NoError(t, core.Sync())

	// 编码失败：Write 返回错误
	broken := &remoteCore{level: zapcore.InfoLevel, enc: errEncoder{newEncoder("json")},
		remoteShared: core.(*remoteCore).remoteShared}
	assert.Error(t, broken.Write(
		zapcore.Entry{Level: zapcore.InfoLevel, Time: time.Now()}, nil))
}

type internalSink struct {
	mu    sync.Mutex
	lines []string
}

func (s *internalSink) got() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.lines)
}

func (s *internalSink) last() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lines[len(s.lines)-1]
}

func (s *internalSink) Flush(lines [][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, l := range lines {
		s.lines = append(s.lines, string(l))
	}
	return nil
}

func TestLineEncoderCloneCarriesFields(t *testing.T) {
	enc := LineEncoder(func(r Rec) string { return fmt.Sprint(r.Msg, " ", r.KV) })
	enc.AddString("svc", "A") // 累积字段（With 路径）
	clone := enc.Clone().(*lineEncoder)
	clone.AddString("extra", "v")

	buf, err := clone.EncodeEntry(
		zapcore.Entry{Level: zapcore.InfoLevel, Message: "m"}, nil)
	require.NoError(t, err)
	got := buf.String()
	assert.Contains(t, got, "svc A", "克隆携带累积字段")
	assert.Contains(t, got, "extra v", "克隆后追加字段生效")
	buf.Free()
}
