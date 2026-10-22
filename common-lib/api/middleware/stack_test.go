package middleware_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/api/internal/testutil"
	"github.com/fab-viewer/common-lib/api/middleware/enrich"
	"github.com/fab-viewer/common-lib/api/middleware/fault"
	"github.com/fab-viewer/common-lib/api/middleware/guard"
	"github.com/fab-viewer/common-lib/api/middleware/observer"
	"github.com/fab-viewer/common-lib/api/response"
	"github.com/fab-viewer/common-lib/common/bizerror"
)

// TestFullStack 验证推荐的注册顺序下各中间件协作：
// accesslog → errorhandler → recovery → runtimecontext → 路由。
func TestFullStack(t *testing.T) {
	log := testutil.CaptureLog(t)
	r := testutil.NewTestEngine()
	r.Use(
		observer.Observe(),
		fault.Fault(),
		guard.Guard(),
		enrich.Enrich(),
	)
	r.GET("/ok", func(c *gin.Context) { response.OK(c, gin.H{"x": 1}) })
	r.GET("/fail", func(c *gin.Context) {
		_ = c.Error(bizerror.NewError(bizerror.CodeNotFound, "user not found"))
	})
	r.GET("/panic", func(_ *gin.Context) { panic("stack boom") })

	t.Run("成功路径", func(t *testing.T) {
		w := testutil.DoGet(r, "/ok")
		require.Equal(t, http.StatusOK, w.Code)
		assert.NotEmpty(t, w.Header().Get(enrich.HeaderTxID))

		var b struct {
			Code    string `json:"code"`
			Success bool   `json:"success"`
			TxID    string `json:"txId"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
		assert.Equal(t, "OK", b.Code, "OK 默认业务码 OK")
		assert.True(t, b.Success)
		assert.Equal(t, w.Header().Get(enrich.HeaderTxID), b.TxID,
			"响应体 txId 与响应头一致")
	})

	t.Run("错误路径", func(t *testing.T) {
		w := testutil.DoGet(r, "/fail")
		require.Equal(t, http.StatusOK, w.Code, "HTTP 恒 200，业务码进 body")
	})

	t.Run("panic 路径", func(t *testing.T) {
		w := testutil.DoGet(r, "/panic")
		require.Equal(t, http.StatusOK, w.Code)
		assert.NotEmpty(t, w.Header().Get(enrich.HeaderTxID),
			"panic 响应仍应带 txId 头")

		var b struct {
			Code    string `json:"code"`
			Success bool   `json:"success"`
			TxID    string `json:"txId"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
		assert.Equal(t, "FAIL", b.Code, "Guard 走 Fail 默认业务码 FAIL")
		assert.False(t, b.Success)
		assert.NotEmpty(t, b.TxID, "Recovery 写回的统一体含 txId")
	})

	// 3 次请求各产生一条访问日志（含 panic 请求），且记录的是最终状态码；
	// 另有 1 条 panic recovered 错误日志与 1 条 business error 信息日志
	records := log.Snapshot()
	accessCount := 0
	statusByPath := map[string]int{}
	for _, rec := range records {
		if rec.Msg == "panic recovered" || rec.Msg == "request fault" {
			continue
		}
		require.Equal(t, "access", rec.Msg)
		accessCount++
		path, _ := testutil.KVGet(rec.KV, "path")
		status, _ := testutil.KVGet(rec.KV, "status")
		if p, ok := path.(string); ok {
			if s, ok := status.(int); ok {
				statusByPath[p] = s
			}
		}
	}
	assert.Equal(t, 3, accessCount, "每次请求一条访问日志（含 panic 请求）")
	assert.Equal(t, http.StatusOK, statusByPath["/ok"])
	assert.Equal(t, http.StatusOK, statusByPath["/fail"],
		"HTTP 恒 200：错误路径也写回 200")
	assert.Equal(t, http.StatusOK, statusByPath["/panic"],
		"panic 路径同样写回 200")
}
