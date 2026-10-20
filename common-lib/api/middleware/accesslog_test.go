package middleware_test

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/api/middleware"
	"github.com/fab-viewer/common-lib/common/logx"
)

func TestAccessLogRecords(t *testing.T) {
	log := &captureLogger{}
	r := newTestEngine()
	r.Use(middleware.AccessLog(log))
	r.GET("/data", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := doGet(r, "/data")
	require.Equal(t, http.StatusOK, w.Code)

	records := log.snapshot()
	require.Len(t, records, 1, "恰好一条访问日志")
	rec := records[0]
	assert.Equal(t, "info", rec.Level)
	assert.Equal(t, "access", rec.Msg)

	for _, key := range []string{"method", "path", "status", "latency_ms", "client_ip", "tx_id", "tx_name", "user", "bytes"} {
		_, ok := kvGet(rec.KV, key)
		assert.True(t, ok, "访问日志缺少字段 %s", key)
	}
	v, _ := kvGet(rec.KV, "method")
	assert.Equal(t, http.MethodGet, v)
	v, _ = kvGet(rec.KV, "path")
	assert.Equal(t, "/data", v)
	v, _ = kvGet(rec.KV, "status")
	assert.Equal(t, http.StatusOK, v)
}

func TestAccessLogSkipsHealthz(t *testing.T) {
	log := &captureLogger{}
	r := newTestEngine()
	r.Use(middleware.AccessLog(log))
	r.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/livez", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/readyz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/api/data", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	doGet(r, "/healthz")
	doGet(r, "/livez")
	doGet(r, "/readyz")
	doGet(r, "/api/data")

	records := log.snapshot()
	require.Len(t, records, 1, "默认跳过 healthz/livez/readyz")
	assert.Equal(t, "/api/data", mustKVString(t, records[0].KV, "path"))
}

func TestAccessLogWithSkipPaths(t *testing.T) {
	log := &captureLogger{}
	r := newTestEngine()
	r.Use(middleware.AccessLog(log, middleware.WithSkipPaths("/ping", "/internal/metrics")))
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/internal/metrics", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/users", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	doGet(r, "/ping")
	doGet(r, "/internal/metrics")
	doGet(r, "/users")

	records := log.snapshot()
	require.Len(t, records, 1)
	assert.Equal(t, "/users", mustKVString(t, records[0].KV, "path"))
}

func TestAccessLogNilLoggerUsesDefault(t *testing.T) {
	capture := &captureLogger{}
	logx.SetDefault(capture)
	t.Cleanup(func() { logx.SetDefault(logx.Nop()) })

	r := newTestEngine()
	r.Use(middleware.AccessLog(nil)) // nil → logx.Default()
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	doGet(r, "/x")
	assert.Len(t, capture.snapshot(), 1, "nil logger 应回退到 logx.Default()")
}

func mustKVString(t *testing.T, kv []any, key string) string {
	t.Helper()
	v, ok := kvGet(kv, key)
	require.True(t, ok, "缺少字段 %s", key)
	s, ok := v.(string)
	require.True(t, ok, "字段 %s 应为字符串", key)
	return s
}
