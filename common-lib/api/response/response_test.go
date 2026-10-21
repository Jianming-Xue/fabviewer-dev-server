package response_test

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/api/internal/testutil"
	"github.com/fab-viewer/common-lib/api/reqctx"
	"github.com/fab-viewer/common-lib/api/response"
	"github.com/fab-viewer/common-lib/common/paging"
	"github.com/fab-viewer/common-lib/common/rctx"
)

func TestOK(t *testing.T) {
	r := testutil.NewTestEngine()
	r.GET("/ok", func(c *gin.Context) {
		response.OK(c, map[string]any{"k": "v"})
	})

	w := testutil.DoGet(r, "/ok")
	require.Equal(t, http.StatusOK, w.Code)

	b := testutil.Decode(t, w)
	assert.Equal(t, http.StatusOK, b.Code, "code 即 HTTP 状态码")
	assert.True(t, b.Success)
	assert.Empty(t, b.ErrMsg, "成功时 errMsg 为空串（字段保留，同构）")
	assert.JSONEq(t, `{"k":"v"}`, string(b.Contents))
	assert.Empty(t, b.TxID, "未启用 TxID 中间件时为空")
}

func TestOKCarriesTxID(t *testing.T) {
	r := testutil.NewTestEngine()
	r.GET("/ok", func(c *gin.Context) {
		c.Set(reqctx.CtxRuntimeContext, rctx.RuntimeContext{}.WithID("tx-123"))
		response.OK(c, nil)
	})

	b := testutil.Decode(t, testutil.DoGet(r, "/ok"))
	assert.Equal(t, "tx-123", b.TxID)
	assert.Equal(t, "null", string(b.Contents), "contents 为 nil 时序列化为 null")
}

func TestCreated(t *testing.T) {
	r := testutil.NewTestEngine()
	r.POST("/res", func(c *gin.Context) { response.Created(c, gin.H{"id": 1}) })

	w := testutil.Do(r, http.MethodPost, "/res")
	require.Equal(t, http.StatusCreated, w.Code)
	b := testutil.Decode(t, w)
	assert.Equal(t, http.StatusCreated, b.Code)
	assert.True(t, b.Success)
	assert.JSONEq(t, `{"id":1}`, string(b.Contents))
}

func TestNoContent(t *testing.T) {
	r := testutil.NewTestEngine()
	r.DELETE("/res/1", func(c *gin.Context) { response.NoContent(c) })

	w := testutil.Do(r, http.MethodDelete, "/res/1")
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String(), "204 无响应体")
}

func TestOKPage(t *testing.T) {
	r := testutil.NewTestEngine()
	r.GET("/list", func(c *gin.Context) {
		response.OKPage(c, paging.NewPageable[string](
			paging.PageRequest{PageIndex: 2, PageSize: 2}, 5, []string{"a", "b"}))
	})

	b := testutil.Decode(t, testutil.DoGet(r, "/list"))
	assert.Equal(t, http.StatusOK, b.Code)
	assert.True(t, b.Success)
	assert.JSONEq(t,
		`{"pageIndex":2,"pageSize":2,"totalSize":5,"totalPage":3,"data":["a","b"]}`, string(b.Contents))
}

func TestOKPageEmptyData(t *testing.T) {
	r := testutil.NewTestEngine()
	r.GET("/empty", func(c *gin.Context) {
		response.OKPage(c, paging.NewPageable[int](paging.PageRequest{}, 0, nil))
	})

	b := testutil.Decode(t, testutil.DoGet(r, "/empty"))
	assert.JSONEq(t, `{"pageIndex":1,"pageSize":10,"totalSize":0,"totalPage":0,"data":[]}`, string(b.Contents),
		"空结果 data 必须是 [] 而非 null")
}

func TestWriteSkipsWhenWritten(t *testing.T) {
	r := testutil.NewTestEngine()
	r.GET("/w", func(c *gin.Context) {
		c.String(http.StatusOK, "first")
		resp := response.NewResponse(c, http.StatusOK, "")
		response.Write(c, http.StatusOK, &resp)
	})

	w := testutil.DoGet(r, "/w")
	assert.Equal(t, "first", w.Body.String(), "已写过的响应不得被覆盖")
}

func TestNoContentSkipsWhenWritten(t *testing.T) {
	r := testutil.NewTestEngine()
	r.GET("/nc", func(c *gin.Context) {
		c.String(http.StatusOK, "first")
		response.NoContent(c)
	})

	w := testutil.DoGet(r, "/nc")
	assert.Equal(t, "first", w.Body.String())
}
