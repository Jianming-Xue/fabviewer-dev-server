package middleware_test

import (
	"errors"
	"net/http"
	"regexp"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/api"
	"github.com/fab-viewer/common-lib/api/middleware"
)

var ridPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

func TestRequestIDGenerate(t *testing.T) {
	r := newTestEngine()
	r.Use(middleware.RequestID())
	r.GET("/echo", func(c *gin.Context) {
		c.String(http.StatusOK, api.RequestIDOf(c))
	})

	w := doRequest(r, http.MethodGet, "/echo")

	rid := w.Header().Get(api.HeaderRequestID)
	require.NotEmpty(t, rid, "响应头必须回写 X-Request-Id")
	assert.Regexp(t, ridPattern, rid, "生成的 ID 为 32 字符小写 hex")
	assert.Equal(t, rid, w.Body.String(), "上下文中的 ID 与响应头一致")
}

func TestRequestIDPassthrough(t *testing.T) {
	r := newTestEngine()
	r.Use(middleware.RequestID())
	r.GET("/echo", func(c *gin.Context) {
		c.String(http.StatusOK, api.RequestIDOf(c))
	})

	w := doRequestWithHeader(r, http.MethodGet, "/echo",
		api.HeaderRequestID, "client-rid-42")

	assert.Equal(t, "client-rid-42", w.Header().Get(api.HeaderRequestID), "透传请求头")
	assert.Equal(t, "client-rid-42", w.Body.String(), "透传上下文")
}

func TestRequestIDUniqueness(t *testing.T) {
	seen := make(map[string]struct{}, 100)
	r := newTestEngine()
	r.Use(middleware.RequestID())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	for range 100 {
		rid := doRequest(r, http.MethodGet, "/x").Header().Get(api.HeaderRequestID)
		seen[rid] = struct{}{}
	}
	assert.Len(t, seen, 100, "100 次请求 ID 不得重复")
}

// 测 crypto/rand 失败的降级分支（注入失败的 randRead）。
func TestNewRequestIDFallback(t *testing.T) {
	orig := middleware.RandReadForTest()
	t.Cleanup(func() { middleware.RestoreRandRead(orig) })

	middleware.SetRandRead(func(_ []byte) (int, error) {
		return 0, errors.New("entropy exhausted")
	})

	rid := middleware.NewRequestIDForTest()
	assert.Regexp(t, regexp.MustCompile(`^t\d+$`), rid, "降级为时间戳前缀")
}
