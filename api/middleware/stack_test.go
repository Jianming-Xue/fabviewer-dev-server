package middleware_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/api"
	"github.com/fab-viewer/common-lib/api/middleware"
	"github.com/fab-viewer/common-lib/common/errs"
)

// TestFullStack 验证推荐的注册顺序下各中间件协作：
// AccessLog → ErrorHandler → Recovery → RuntimeContext → 路由。
func TestFullStack(t *testing.T) {
	log := &captureLogger{}
	r := newTestEngine()
	r.Use(
		middleware.AccessLog(log),
		middleware.ErrorHandler(log),
		middleware.Recovery(log),
		middleware.RuntimeContext(),
	)
	r.GET("/ok", func(c *gin.Context) { api.OK(c, gin.H{"x": 1}) })
	r.GET("/fail", func(c *gin.Context) {
		_ = c.Error(errs.New(errs.CodeNotFound, "user not found"))
	})
	r.GET("/panic", func(_ *gin.Context) { panic("stack boom") })

	t.Run("成功路径", func(t *testing.T) {
		w := doGet(r, "/ok")
		require.Equal(t, http.StatusOK, w.Code)
		assert.NotEmpty(t, w.Header().Get(api.HeaderTxID))

		var b struct {
			Code    int    `json:"code"`
			Success bool   `json:"success"`
			TxID    string `json:"txId"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
		assert.Equal(t, http.StatusOK, b.Code)
		assert.True(t, b.Success)
		assert.Equal(t, w.Header().Get(api.HeaderTxID), b.TxID,
			"响应体 txId 与响应头一致")
	})

	t.Run("错误路径", func(t *testing.T) {
		w := doGet(r, "/fail")
		require.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("panic 路径", func(t *testing.T) {
		w := doGet(r, "/panic")
		require.Equal(t, http.StatusInternalServerError, w.Code)
		assert.NotEmpty(t, w.Header().Get(api.HeaderTxID),
			"panic 响应仍应带 txId 头")

		var b struct {
			Code    int    `json:"code"`
			Success bool   `json:"success"`
			TxID    string `json:"txId"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
		assert.Equal(t, http.StatusInternalServerError, b.Code)
		assert.False(t, b.Success)
		assert.NotEmpty(t, b.TxID, "Recovery 写回的统一体含 txId")
	})

	// 3 次请求各产生一条访问日志（含 panic 请求），且记录的是最终状态码；
	// 另有 1 条 panic recovered 错误日志与 1 条 business error 信息日志
	records := log.snapshot()
	accessCount := 0
	statusByPath := map[string]int{}
	for _, rec := range records {
		if rec.Msg == "panic recovered" || rec.Msg == "business error" {
			continue
		}
		require.Equal(t, "access", rec.Msg)
		accessCount++
		path, _ := kvGet(rec.KV, "path")
		status, _ := kvGet(rec.KV, "status")
		if p, ok := path.(string); ok {
			if s, ok := status.(int); ok {
				statusByPath[p] = s
			}
		}
	}
	assert.Equal(t, 3, accessCount, "每次请求一条访问日志（含 panic 请求）")
	assert.Equal(t, http.StatusOK, statusByPath["/ok"])
	assert.Equal(t, http.StatusNotFound, statusByPath["/fail"],
		"访问日志必须记录 ErrorHandler 写回的最终状态码")
	assert.Equal(t, http.StatusInternalServerError, statusByPath["/panic"],
		"访问日志必须记录 Recovery 写回的最终状态码")
}
