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
	r := server.NewEngine(map[bizerror.Code]int{
		bizerror.Code("USER_DISABLED"): http.StatusForbidden,
	})
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

	t.Run("模块码按 codeMap 映射进响应体（HTTP 恒 200）", func(t *testing.T) {
		w := doGet(r, "/disabled")
		require.Equal(t, http.StatusOK, w.Code)
		var b struct {
			Code   int    `json:"code"`
			ErrMsg string `json:"errMsg"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
		assert.Equal(t, http.StatusForbidden, b.Code, "映射值进 body 的业务 code")
		assert.Equal(t, "账号已禁用", b.ErrMsg)
	})

	t.Run("未知错误兜底 code 0 统一体", func(t *testing.T) {
		w := doGet(r, "/unknown")
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"code":0`)
		assert.NotContains(t, w.Body.String(), "assert.AnError", "根因不进响应")
	})

	t.Run("panic 兜底 code 0 且响应头带 txId", func(t *testing.T) {
		w := doGet(r, "/boom")
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"code":0`)
		assert.NotEmpty(t, w.Header().Get("txId"), "enrich 中间件已入链")
	})
}

func TestNewEngineNilCodeMap(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := server.NewEngine(nil)
	r.GET("/e", func(c *gin.Context) {
		_ = c.Error(bizerror.NewError(bizerror.CodeNotFound, "user not found"))
	})

	w := doGet(r, "/e")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"code":0`, "nil 码表按 Fail 默认 0")
}

func TestNewEngineWithMessageFunc(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := server.NewEngine(nil, server.WithMessageFunc(
		func(_ *gin.Context, _ *bizerror.BizError) string { return "overridden" }))
	r.GET("/e", func(c *gin.Context) {
		_ = c.Error(bizerror.NewError(bizerror.CodeNotFound, "raw"))
	})

	var b struct {
		ErrMsg string `json:"errMsg"`
	}
	require.NoError(t, json.Unmarshal(doGet(r, "/e").Body.Bytes(), &b))
	assert.Equal(t, "overridden", b.ErrMsg)
}

func TestNewEngineWithSkipPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := server.NewEngine(nil, server.WithSkipPaths("/metrics"))
	r.GET("/metrics", func(c *gin.Context) { c.String(http.StatusOK, "m") })

	assert.Equal(t, http.StatusOK, doGet(r, "/metrics").Code, "skipPaths 只影响日志，不影响路由")
}
