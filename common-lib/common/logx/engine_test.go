package logx_test

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/common/logx"
)

// fakeSink 收集 Flush 到的批次（可注入阻塞）。
type fakeSink struct {
	mu      sync.Mutex
	batches [][][]byte
	block   chan struct{}
}

func (f *fakeSink) Flush(lines [][]byte) error {
	if f.block != nil {
		<-f.block // 测试用阻塞
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.batches = append(f.batches, lines)
	return nil
}

func (f *fakeSink) snapshot() (out [][][]byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append(out, f.batches...)
}

func (f *fakeSink) totalLines() int {
	n := 0
	for _, b := range f.snapshot() {
		n += len(b)
	}
	return n
}

func TestNewFileOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	log, closeLog, err := logx.New(logx.Config{
		Level: "info",
		File:  &logx.FileConfig{Path: path},
	})
	require.NoError(t, err)

	log.Debug("应被级别过滤")
	log.Info("access", "tx_id", "t-1", "status", 404)
	log.Error("boom", "err", "timeout")
	log.With("svc", "demo").Info("with 派生")
	closeLog()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	got := string(data)
	assert.Contains(t, got, `"msg":"access"`)
	assert.Contains(t, got, `"tx_id":"t-1"`)
	assert.Contains(t, got, `"status":404`)
	assert.Contains(t, got, `"msg":"boom"`)
	assert.Contains(t, got, `"err":"timeout"`)
	assert.Contains(t, got, `"svc":"demo"`, "With 字段随行携带")
	assert.NotContains(t, got, "应被级别过滤", "info 级别下 debug 不输出")
	assert.Contains(t, got, `"level":"info"`, "级别小写")
}

func TestNewConsoleFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	log, closeLog, err := logx.New(logx.Config{
		Format: "console",
		File:   &logx.FileConfig{Path: path},
	})
	require.NoError(t, err)
	log.Info("hello", "k", "v")
	closeLog()

	data, _ := os.ReadFile(path)
	assert.NotContains(t, string(data), `"msg"`, "console 格式非 JSON")
	assert.Contains(t, string(data), "hello")
}

func TestNewFileRotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	log, closeLog, err := logx.New(logx.Config{
		File: &logx.FileConfig{Path: path, MaxSizeMB: 1, MaxBackups: 2},
	})
	require.NoError(t, err)

	// 写超 1MB 触发编号滚动（每行 ~120B，1.2 万行足够）
	for i := range 12000 {
		log.Info("rotation fill line", "i", i)
	}
	closeLog()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name()] = true
	}
	assert.True(t, names["app.log.1"], "滚动产生 app.log.1")
	assert.LessOrEqual(t, countBackups(names), 2, "留档不超过 MaxBackups")
}

func countBackups(names map[string]bool) int {
	n := 0
	for name := range names {
		if name != "app.log" {
			n++
		}
	}
	return n
}

func TestNewPerSinkEncoders(t *testing.T) {
	// 场景同业务服务 A：文件出口为业务自定义字符串格式，
	// 远程出口为业务自定义 JSON（两条格式互不相同）
	path := filepath.Join(t.TempDir(), "app.log")
	sink := &fakeSink{}
	log, closeLog, err := logx.New(logx.Config{
		File: &logx.FileConfig{
			Path: path,
			Encoder: logx.LineEncoder(func(r logx.Rec) string {
				return fmt.Sprintf("%s|%s|%s|%v",
					r.Time.Format("2006-01-02 15:04:05"), r.Level, r.Msg, r.KV)
			}),
		},
		Remote: sink,
		RemoteEncoder: logx.LineEncoder(func(r logx.Rec) string {
			return fmt.Sprintf(`{"lvl":%q,"msg":%q,"kv":%d}`,
				r.Level, r.Msg, len(r.KV)/2)
		}),
		RemoteBatchSize:  10,
		RemoteFlushEvery: time.Hour,
	})
	require.NoError(t, err)

	log.With("svc", "A").Info("hello", "tx_id", "t-1")
	closeLog()

	data, _ := os.ReadFile(path)
	got := string(data)
	assert.Contains(t, got, "|info|hello|", "文件出口为业务字符串格式")
	assert.Contains(t, got, "svc A", "With 派生字段随行")

	batches := sink.snapshot()
	require.Len(t, batches, 1)
	assert.Contains(t, string(batches[0][0]), `"lvl":"info"`, "远程出口为业务 JSON 格式")
	assert.Contains(t, string(batches[0][0]), `"kv":2`)
}

func TestNewRemoteBatches(t *testing.T) {
	sink := &fakeSink{}
	log, closeLog, err := logx.New(logx.Config{
		Stdout:           false,
		Remote:           sink,
		RemoteBatchSize:  2,
		RemoteFlushEvery: time.Hour, // 只靠凑批触发
	})
	require.NoError(t, err)

	for i := range 5 {
		log.Info("line", "i", i)
	}
	closeLog()

	batches := sink.snapshot()
	require.Len(t, batches, 3, "5 行按批 2 切成 2+2+1")
	assert.Len(t, batches[0], 2)
	assert.Len(t, batches[2], 1)
	assert.Contains(t, string(batches[0][0]), `"msg":"line"`)
}

func TestNewRemoteFlushByInterval(t *testing.T) {
	sink := &fakeSink{}
	log, closeLog, err := logx.New(logx.Config{
		Remote:           sink,
		RemoteBatchSize:  100, // 凑不满
		RemoteFlushEvery: 50 * time.Millisecond,
	})
	require.NoError(t, err)

	log.Info("tick line")
	require.Eventually(t, func() bool { return sink.totalLines() == 1 },
		2*time.Second, 10*time.Millisecond, "定时刷出应独立于凑批")
	closeLog()
	assert.Equal(t, 1, sink.totalLines())
}

func TestNewRemoteDropWhenBufferFull(t *testing.T) {
	sink := &fakeSink{block: make(chan struct{})}
	log, closeLog, err := logx.New(logx.Config{
		Remote:           sink,
		RemoteBatchSize:  1,
		RemoteFlushEvery: time.Hour,
		RemoteBufferSize: 2,
	})
	require.NoError(t, err)

	// Flush 阻塞期间继续写：缓冲满后丢弃，绝不阻塞调用方
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 10 {
			log.Info("flood", "i", i)
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("缓冲满时写入被阻塞")
	}
	close(sink.block) // 放行
	closeLog()

	assert.Less(t, sink.totalLines(), 10, "超出缓冲的行被丢弃")
	assert.GreaterOrEqual(t, sink.totalLines(), 1)
}

func TestNewStdoutOutput(t *testing.T) {
	// 捕获标准输出验证 stdout 出口
	orig := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = orig })

	log, closeLog, err := logx.New(logx.Config{Stdout: true})
	require.NoError(t, err)
	log.Info("to stdout", "k", "v")
	log.Warn("warned") // 补 Warn 分支
	closeLog()
	_ = w.Close()

	out, _ := io.ReadAll(r)
	assert.Contains(t, string(out), `"msg":"to stdout"`)
	assert.Contains(t, string(out), `"msg":"warned"`)
}

// errSink 投递必失败（测丢弃计数路径）。
type errSink struct {
	mu    sync.Mutex
	calls int
}

func (e *errSink) Flush(_ [][]byte) error {
	e.mu.Lock()
	e.calls++
	e.mu.Unlock()
	return errors.New("remote down")
}

func TestNewRemoteFlushErrorDrops(t *testing.T) {
	sink := &errSink{}
	log, closeLog, err := logx.New(logx.Config{
		Remote:           sink,
		RemoteBatchSize:  1,
		RemoteFlushEvery: 10 * time.Millisecond,
	})
	require.NoError(t, err)
	log.Info("lost line")
	closeLog()
	assert.GreaterOrEqual(t, sink.calls, 1, "失败批被投递尝试")
}

func TestNewRemoteDefaultFlushEvery(t *testing.T) {
	sink := &fakeSink{}
	// 不传 RemoteFlushEvery：走默认值分支，构造即覆盖
	log, closeLog, err := logx.New(logx.Config{Remote: sink, RemoteBatchSize: 1})
	require.NoError(t, err)
	log.Info("line")
	require.Eventually(t, func() bool { return sink.totalLines() == 1 },
		6*time.Second, 10*time.Millisecond, "默认 5s 定时也应在测试窗口内刷出")
	closeLog()
}

func TestNewConfigErrors(t *testing.T) {
	_, _, err := logx.New(logx.Config{})
	assert.ErrorContains(t, err, "至少启用一个出口")

	_, _, err = logx.New(logx.Config{Level: "loud", File: &logx.FileConfig{Path: "x"}})
	assert.ErrorContains(t, err, "非法级别")

	dir := t.TempDir()
	_, _, err = logx.New(logx.Config{File: &logx.FileConfig{}})
	assert.ErrorContains(t, err, "Path 不能为空")

	// Path 指向目录：文件出口打开失败向上传播
	_, _, err = logx.New(logx.Config{File: &logx.FileConfig{Path: dir}})
	assert.ErrorContains(t, err, "打开日志文件")
}
