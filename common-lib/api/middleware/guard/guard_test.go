package guard_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/api/internal/testutil"
	"github.com/fab-viewer/common-lib/api/middleware/guard"
)

func TestGuardPanickingHandler(t *testing.T) {
	log := testutil.CaptureLog(t)
	r := testutil.NewTestEngine()
	r.Use(guard.Guard())
	r.GET("/boom", func(_ *gin.Context) { panic("something exploded") })

	w := testutil.DoGet(r, "/boom")
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Code    string `json:"code"`
		Success bool   `json:"success"`
		ErrMsg  string `json:"errMsg"`
		Data    any    `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "InternalError", body.Code, "panic 路径走公共错误 InternalError")
	assert.Nil(t, body.Data)
	assert.False(t, body.Success)
	assert.Equal(t, "internal error", body.ErrMsg, "默认英文；不向调用方泄漏 panic 内容")

	records := log.Snapshot()
	require.Len(t, records, 1)
	rec := records[0]
	assert.Equal(t, "error", rec.Level)
	assert.Equal(t, "panic recovered", rec.Msg)
	v, ok := testutil.KVGet(rec.KV, "error")
	require.True(t, ok)
	assert.Equal(t, "something exploded", v)
	_, ok = testutil.KVGet(rec.KV, "stack")
	assert.True(t, ok, "panic 日志必须带堆栈")
}

func TestGuardAfterPartialWrite(t *testing.T) {
	log := testutil.CaptureLog(t)
	r := testutil.NewTestEngine()
	r.Use(guard.Guard())
	r.GET("/partial", func(c *gin.Context) {
		c.String(http.StatusOK, "partial response")
		panic("late panic")
	})

	w := testutil.DoGet(r, "/partial")
	assert.Equal(t, "partial response", w.Body.String(), "已写部分响应不得被二次写")
	assert.Len(t, log.Snapshot(), 1, "panic 仍应记录日志")
}

func TestGuardNormalPassThrough(t *testing.T) {
	log := testutil.CaptureLog(t)
	r := testutil.NewTestEngine()
	r.Use(guard.Guard())
	r.GET("/ok", func(c *gin.Context) { c.String(http.StatusOK, "fine") })

	w := testutil.DoGet(r, "/ok")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "fine", w.Body.String())
	assert.Empty(t, log.Snapshot(), "无 panic 不产生日志")
}
