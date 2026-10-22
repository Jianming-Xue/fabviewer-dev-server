package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	server "github.com/fab-viewer/common-lib/cmd/server"
	"github.com/fab-viewer/common-lib/common/bizerror"
)

func doGet(r http.Handler, target string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, target, http.NoBody)
	r.ServeHTTP(w, req)
	return w
}

func TestNewEngine(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := server.NewEngine()
	r.GET("/disabled", func(c *gin.Context) {
		_ = c.Error(bizerror.NewError(bizerror.Code("USER_DISABLED"), "账号已禁用"))
	})
	r.GET("/unknown", func(c *gin.Context) { _ = c.Error(assert.AnError) })
	r.GET("/boom", func(_ *gin.Context) { panic("engine boom") })

	t.Run("内置 healthz 探活", func(t *testing.T) {
		w := doGet(r, "/healthz")
		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "ok", w.Body.String())
	})

	t.Run("内置 /api/readyz 就绪探活：OK 无数据", func(t *testing.T) {
		w := doGet(r, "/api/readyz")
		require.Equal(t, http.StatusOK, w.Code)
		var b struct {
			Code string `json:"code"`
			Data any    `json:"data"`
			TxID string `json:"txId"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
		assert.Equal(t, "OK", b.Code)
		assert.Nil(t, b.Data, "不带任何数据")
		assert.NotEmpty(t, b.TxID)
	})

	t.Run("BizError 的 code 透传进响应体（HTTP 恒 200）", func(t *testing.T) {
		w := doGet(r, "/disabled")
		require.Equal(t, http.StatusOK, w.Code)
		var b struct {
			Code   string `json:"code"`
			ErrMsg string `json:"errMsg"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
		assert.Equal(t, "USER_DISABLED", b.Code, "bizerror.Code 字符串直接透传")
		assert.Equal(t, "账号已禁用", b.ErrMsg)
	})

	t.Run("未知错误兜底 code FAIL 且 errMsg 直出原文", func(t *testing.T) {
		w := doGet(r, "/unknown")
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"code":"FAIL"`)
		assert.Contains(t, w.Body.String(), assert.AnError.Error(), "err 原文直出响应体")
	})

	t.Run("panic 兜底公共错误 InternalError 且响应头带 txId", func(t *testing.T) {
		w := doGet(r, "/boom")
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"code":"InternalError"`)
		assert.NotEmpty(t, w.Header().Get("txId"), "enrich 中间件已入链")
	})
}

func TestNewEngineBizErrorCodePassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := server.NewEngine()
	r.GET("/e", func(c *gin.Context) {
		_ = c.Error(bizerror.NewError(bizerror.CodeNotFound, "user not found"))
	})

	w := doGet(r, "/e")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"code":"NOT_FOUND"`, "bizerror.Code 直接透传")
}

func TestNewEngineWithSkipPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := server.NewEngine(server.WithSkipPaths("/metrics"))
	r.GET("/metrics", func(c *gin.Context) { c.String(http.StatusOK, "m") })

	assert.Equal(t, http.StatusOK, doGet(r, "/metrics").Code, "skipPaths 只影响日志，不影响路由")
}
