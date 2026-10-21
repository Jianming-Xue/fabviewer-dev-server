package reqctx_test

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/api/internal/testutil"
	"github.com/fab-viewer/common-lib/api/reqctx"
	"github.com/fab-viewer/common-lib/common/rctx"
)

func TestRuntimeContextOfZeroValueWithoutMiddleware(t *testing.T) {
	r := testutil.NewTestEngine()
	r.GET("/rc", func(c *gin.Context) {
		rc := reqctx.RuntimeContextOf(c)
		assert.Equal(t, rctx.RuntimeContext{}, rc, "未启用中间件时为零值")
		c.String(http.StatusOK, "ok")
	})

	require.Equal(t, http.StatusOK, testutil.DoGet(r, "/rc").Code)
}

func TestTxIDOfAndLangOfReadInjectedContext(t *testing.T) {
	r := testutil.NewTestEngine()
	r.GET("/echo", func(c *gin.Context) {
		c.Set(reqctx.CtxRuntimeContext, rctx.RuntimeContext{}.WithID("tx-9").WithLang("zh-CN"))
		assert.Equal(t, "tx-9", reqctx.TxIDOf(c), "TxIDOf 是 RuntimeContextOf(c).FromID() 的语法糖")
		c.String(http.StatusOK, reqctx.LangOf(c))
	})

	w := testutil.DoGet(r, "/echo")
	assert.Equal(t, "zh-CN", w.Body.String(), "LangOf 是 RuntimeContextOf(c).FromLang() 的语法糖")
}

func TestWithActorWritesRequestContext(t *testing.T) {
	var user string
	r := testutil.NewTestEngine()
	r.GET("/auth", func(c *gin.Context) {
		reqctx.WithActor(c, "u-77")
		user = rctx.GetRuntimeContext(c.Request.Context()).FromUser()
	})

	testutil.DoGet(r, "/auth")
	assert.Equal(t, "u-77", user, "WithActor 经 rctx 派生 WithUser 写入请求 context")
}
