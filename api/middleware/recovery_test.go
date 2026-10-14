package middleware_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/api/middleware"
	"github.com/fab-viewer/common-lib/logx"
)

func TestRecoveryPanickingHandler(t *testing.T) {
	log := &captureLogger{}
	r := newTestEngine()
	r.Use(middleware.Recovery(log))
	r.GET("/boom", func(_ *gin.Context) { panic("something exploded") })

	w := doRequest(r, http.MethodGet, "/boom")
	require.Equal(t, http.StatusInternalServerError, w.Code)

	var body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "INTERNAL_ERROR", body.Code)
	assert.Equal(t, "internal error", body.Message, "不向调用方泄漏 panic 内容")

	records := log.snapshot()
	require.Len(t, records, 1)
	rec := records[0]
	assert.Equal(t, "error", rec.Level)
	assert.Equal(t, "panic recovered", rec.Msg)
	v, ok := kvGet(rec.KV, "error")
	require.True(t, ok)
	assert.Equal(t, "something exploded", v)
	_, ok = kvGet(rec.KV, "stack")
	assert.True(t, ok, "panic 日志必须带堆栈")
}

func TestRecoveryAfterPartialWrite(t *testing.T) {
	log := &captureLogger{}
	r := newTestEngine()
	r.Use(middleware.Recovery(log))
	r.GET("/partial", func(c *gin.Context) {
		c.String(http.StatusOK, "partial response")
		panic("late panic")
	})

	w := doRequest(r, http.MethodGet, "/partial")
	assert.Equal(t, "partial response", w.Body.String(), "已写部分响应不得被二次写")
	assert.Len(t, log.snapshot(), 1, "panic 仍应记录日志")
}

func TestRecoveryNormalPassThrough(t *testing.T) {
	log := &captureLogger{}
	r := newTestEngine()
	r.Use(middleware.Recovery(log))
	r.GET("/ok", func(c *gin.Context) { c.String(http.StatusOK, "fine") })

	w := doRequest(r, http.MethodGet, "/ok")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "fine", w.Body.String())
	assert.Empty(t, log.snapshot(), "无 panic 不产生日志")
}

func TestRecoveryNilLoggerUsesDefault(t *testing.T) {
	capture := &captureLogger{}
	logx.SetDefault(capture)
	t.Cleanup(func() { logx.SetDefault(logx.Nop()) })

	r := newTestEngine()
	r.Use(middleware.Recovery(nil)) // nil → logx.Default()
	r.GET("/boom", func(_ *gin.Context) { panic("x") })

	w := doRequest(r, http.MethodGet, "/boom")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Len(t, capture.snapshot(), 1, "nil logger 应回退到 logx.Default()")
}
