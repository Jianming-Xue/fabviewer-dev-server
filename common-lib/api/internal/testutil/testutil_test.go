package testutil_test

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/api/internal/testutil"
)

func TestCaptureLog(t *testing.T) {
	l := testutil.CaptureLog(t)
	l.Debug("d")
	l.Warn("w", "k", "v")
	records := l.Snapshot()
	require.Len(t, records, 2)
	assert.Equal(t, "debug", records[0].Level)
	assert.Equal(t, "warn", records[1].Level)
	v, ok := testutil.KVGet(records[1].KV, "k")
	require.True(t, ok)
	assert.Equal(t, "v", v)
	// 恢复由 t.Cleanup 完成（测试结束回到 Nop）
}

func TestCaptureLogger(t *testing.T) {
	l := &testutil.CaptureLogger{}
	l.Info("hello", "k", "v")
	l.With("x", 1).Error("boom") // With 返回自身，同一实例继续收集

	records := l.Snapshot()
	require.Len(t, records, 2)
	assert.Equal(t, "info", records[0].Level)
	assert.Equal(t, "error", records[1].Level)

	v, ok := testutil.KVGet(records[0].KV, "k")
	require.True(t, ok)
	assert.Equal(t, "v", v)
	_, ok = testutil.KVGet(records[0].KV, "missing")
	assert.False(t, ok, "不存在的键返回 false")

	assert.Equal(t, "hello", records[0].Msg)
	assert.Equal(t, "v", testutil.MustKVString(t, records[0].KV, "k"))
}

func TestEngineAndRequests(t *testing.T) {
	r := testutil.NewTestEngine()
	assert.Equal(t, gin.TestMode, gin.Mode())
	r.GET("/e", func(c *gin.Context) {
		c.Header("x-name", c.GetHeader("x-name"))
		c.String(http.StatusOK, "ok")
	})

	w := testutil.DoGet(r, "/e")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())

	w2 := testutil.DoGetWithHeaders(r, "/e", map[string]string{"x-name": "abc"})
	assert.Equal(t, "abc", w2.Header().Get("x-name"), "请求头应透传到响应头")
}

func TestDecode(t *testing.T) {
	r := testutil.NewTestEngine()
	r.GET("/json", func(c *gin.Context) { c.String(http.StatusOK, `{"code":200}`) })

	b := testutil.Decode(t, testutil.DoGet(r, "/json"))
	assert.Equal(t, 200, b.Code)
	assert.False(t, b.Success)
}
