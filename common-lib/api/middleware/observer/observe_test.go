package observer_test

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/api/internal/testutil"
	"github.com/fab-viewer/common-lib/api/middleware/observer"
)

func TestObserveRecords(t *testing.T) {
	log := testutil.CaptureLog(t)
	r := testutil.NewTestEngine()
	r.Use(observer.Observe())
	r.GET("/data", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := testutil.DoGet(r, "/data")
	require.Equal(t, http.StatusOK, w.Code)

	records := log.Snapshot()
	require.Len(t, records, 1, "恰好一条访问日志")
	rec := records[0]
	assert.Equal(t, "info", rec.Level)
	assert.Equal(t, "access", rec.Msg)

	for _, key := range []string{"method", "path", "status", "latency_ms", "client_ip", "tx_id", "tx_name", "user", "bytes"} {
		_, ok := testutil.KVGet(rec.KV, key)
		assert.True(t, ok, "访问日志缺少字段 %s", key)
	}
	v, _ := testutil.KVGet(rec.KV, "method")
	assert.Equal(t, http.MethodGet, v)
	v, _ = testutil.KVGet(rec.KV, "path")
	assert.Equal(t, "/data", v)
	v, _ = testutil.KVGet(rec.KV, "status")
	assert.Equal(t, http.StatusOK, v)
}

func TestObserveSkipsHealthz(t *testing.T) {
	log := testutil.CaptureLog(t)
	r := testutil.NewTestEngine()
	r.Use(observer.Observe())
	r.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/livez", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/readyz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/api/data", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	testutil.DoGet(r, "/healthz")
	testutil.DoGet(r, "/livez")
	testutil.DoGet(r, "/readyz")
	testutil.DoGet(r, "/api/data")

	records := log.Snapshot()
	require.Len(t, records, 1, "默认跳过 healthz/livez/readyz")
	assert.Equal(t, "/api/data", testutil.MustKVString(t, records[0].KV, "path"))
}

func TestObserveWithSkipPaths(t *testing.T) {
	log := testutil.CaptureLog(t)
	r := testutil.NewTestEngine()
	r.Use(observer.Observe(observer.WithSkipPaths("/ping", "/internal/metrics")))
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/internal/metrics", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/users", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	testutil.DoGet(r, "/ping")
	testutil.DoGet(r, "/internal/metrics")
	testutil.DoGet(r, "/users")

	records := log.Snapshot()
	require.Len(t, records, 1)
	assert.Equal(t, "/users", testutil.MustKVString(t, records[0].KV, "path"))
}
