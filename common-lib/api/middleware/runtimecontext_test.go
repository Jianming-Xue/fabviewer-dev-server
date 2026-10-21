package middleware_test

import (
	"errors"
	"net/http"
	"regexp"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/api"
	"github.com/fab-viewer/common-lib/api/middleware"
	"github.com/fab-viewer/common-lib/common/rctx"
)

var txidPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

func TestRuntimeContextFullHeaders(t *testing.T) {
	var got rctx.RuntimeContext
	r := newTestEngine()
	r.Use(middleware.FillRuntimeContext())
	r.GET("/echo", func(c *gin.Context) {
		got = rctx.GetRuntimeContext(c.Request.Context())
		c.String(http.StatusOK, got.FromID())
	})

	reqTime := "20261019173000"
	w := doGetWithHeaders(r, "/echo", map[string]string{
		api.HeaderTxID:           "tx-42",
		api.HeaderTxTime:         reqTime,
		api.HeaderTxName:         "queryUsers",
		api.HeaderUserName:       "zhangsan",
		api.HeaderAcceptLanguage: "zh-CN,zh;q=0.9,en;q=0.8",
	})

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, rctx.RuntimeContext{}.
		WithID("tx-42").
		WithTime(time.Date(2026, 10, 19, 17, 30, 0, 0, time.UTC)).
		WithName("queryUsers").
		WithUser("zhangsan").
		WithLang("zh-CN"), got, "五字段完整解析进 tx 上下文")
	assert.Equal(t, "tx-42", w.Body.String(), "请求 context 可读")
	assert.Equal(t, "tx-42", w.Header().Get(api.HeaderTxID), "响应头回写 txId")
}

func TestRuntimeContextLanguage(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"q 值降序取首选", "en;q=0.8,zh-CN", "zh-CN"},
		{"规范化大小写与下划线", "EN_us", "en-US"},
		{"缺失为空串", "", ""},
		{"通配符不产生语言", "*,en;q=0", ""},
		{"全非法为空串", "!!!,;;q=", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got rctx.RuntimeContext
			r := newTestEngine()
			r.Use(middleware.FillRuntimeContext())
			r.GET("/e", func(c *gin.Context) {
				got = rctx.GetRuntimeContext(c.Request.Context())
				assert.Equal(t, got.FromLang(), api.LangOf(c), "LangOf 是 RuntimeContextOf(c).Lang 的语法糖")
			})

			doGetWithHeaders(r, "/e", map[string]string{api.HeaderAcceptLanguage: tt.header})
			assert.Equal(t, tt.want, got.FromLang())
		})
	}
}

func TestRuntimeContextOfInHandler(t *testing.T) {
	r := newTestEngine()
	r.Use(middleware.FillRuntimeContext())
	r.GET("/gin-side", func(c *gin.Context) {
		g := api.RuntimeContextOf(c)
		assert.Equal(t, "tx-7", g.FromID())
		assert.Equal(t, "listOrders", g.FromName())
		c.String(http.StatusOK, api.TxIDOf(c))
	})

	w := doGetWithHeaders(r, "/gin-side", map[string]string{
		api.HeaderTxID:   "tx-7",
		api.HeaderTxName: "listOrders",
	})
	assert.Equal(t, "tx-7", w.Body.String(), "TxIDOf 是 RuntimeContextOf(c).ID 的语法糖")
}

func TestRuntimeContextTxTimeFallback(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{"缺失兜底服务器时间", ""},
		{"非法格式兜底服务器时间", "2026/10/19 17:30"},
		{"位数不对兜底服务器时间", "2026101917"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got rctx.RuntimeContext
			before := time.Now().Add(-time.Second)
			r := newTestEngine()
			r.Use(middleware.FillRuntimeContext())
			r.GET("/e", func(c *gin.Context) { got = rctx.GetRuntimeContext(c.Request.Context()) })

			doGetWithHeaders(r, "/e", map[string]string{
				api.HeaderTxTime: tt.header,
			})

			after := time.Now().Add(time.Second)
			require.False(t, got.FromTime().Before(before) || got.FromTime().After(after),
				"txTime 应兜底为服务器当前时间，实际 %v", got.FromTime())
		})
	}
}

func TestRuntimeContextUserNameToActor(t *testing.T) {
	var actorID string
	r := newTestEngine()
	r.Use(middleware.FillRuntimeContext())
	r.GET("/audit", func(c *gin.Context) {
		actorID = rctx.GetRuntimeContext(c.Request.Context()).FromUser()
	})

	doGetWithHeaders(r, "/audit", map[string]string{
		api.HeaderUserName: "zhangsan",
	})
	assert.Equal(t, "zhangsan", actorID, "userName 即审计操作者（RuntimeContext.User）")

	doGet(r, "/audit")
	assert.Empty(t, actorID, "userName 缺失时操作者为空")
}

func TestRuntimeContextTxIDGenerate(t *testing.T) {
	r := newTestEngine()
	r.Use(middleware.FillRuntimeContext())
	r.GET("/echo", func(c *gin.Context) {
		c.String(http.StatusOK, api.TxIDOf(c))
	})

	w := doGet(r, "/echo")

	id := w.Header().Get(api.HeaderTxID)
	require.NotEmpty(t, id, "响应头必须回写 txId")
	assert.Regexp(t, txidPattern, id, "生成的 ID 为 32 字符小写 hex")
	assert.Equal(t, id, w.Body.String(), "上下文中的 ID 与响应头一致")
}

func TestRuntimeContextTxIDUniqueness(t *testing.T) {
	seen := make(map[string]struct{}, 100)
	r := newTestEngine()
	r.Use(middleware.FillRuntimeContext())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	for range 100 {
		id := doGet(r, "/x").Header().Get(api.HeaderTxID)
		seen[id] = struct{}{}
	}
	assert.Len(t, seen, 100, "100 次请求 ID 不得重复")
}

// 测 crypto/rand 失败的降级分支（注入失败的 randRead）。
func TestNewTxIDFallback(t *testing.T) {
	orig := middleware.RandReadForTest()
	t.Cleanup(func() { middleware.RestoreRandRead(orig) })

	middleware.SetRandRead(func(_ []byte) (int, error) {
		return 0, errors.New("entropy exhausted")
	})

	id := middleware.NewTxIDForTest()
	assert.Regexp(t, regexp.MustCompile(`^t\d+$`), id, "降级为时间戳前缀")
}
