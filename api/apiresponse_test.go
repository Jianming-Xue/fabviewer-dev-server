package api_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/api"
	"github.com/fab-viewer/common-lib/common/errs"
	"github.com/fab-viewer/common-lib/common/paging"
	"github.com/fab-viewer/common-lib/common/tx"
)

// respBody 是统一响应体的测试解码目标（与 api.Response 字段一一对应）。
type respBody struct {
	Code     int             `json:"code"`
	Success  bool            `json:"success"`
	ErrMsg   string          `json:"errMsg"`
	Contents json.RawMessage `json:"contents"`
	TxID     string          `json:"txId"`
}

func newTestEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func doRequest(t *testing.T, r http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), method, target, http.NoBody)
	r.ServeHTTP(w, req)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) respBody {
	t.Helper()
	var b respBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
	return b
}

func TestOK(t *testing.T) {
	r := newTestEngine()
	r.GET("/ok", func(c *gin.Context) {
		api.OK(c, map[string]any{"k": "v"})
	})

	w := doRequest(t, r, http.MethodGet, "/ok")
	require.Equal(t, http.StatusOK, w.Code)

	b := decode(t, w)
	assert.Equal(t, http.StatusOK, b.Code, "code 即 HTTP 状态码")
	assert.True(t, b.Success)
	assert.Empty(t, b.ErrMsg, "成功时 errMsg 为空串（字段保留，同构）")
	assert.JSONEq(t, `{"k":"v"}`, string(b.Contents))
	assert.Empty(t, b.TxID, "未启用 TxID 中间件时为空")
}

func TestOKCarriesTxID(t *testing.T) {
	r := newTestEngine()
	r.GET("/ok", func(c *gin.Context) {
		c.Set(api.CtxTx, tx.Tx{ID: "tx-123"})
		api.OK(c, nil)
	})

	b := decode(t, doRequest(t, r, http.MethodGet, "/ok"))
	assert.Equal(t, "tx-123", b.TxID)
	assert.Equal(t, "null", string(b.Contents), "contents 为 nil 时序列化为 null")
}

func TestCreated(t *testing.T) {
	r := newTestEngine()
	r.POST("/res", func(c *gin.Context) { api.Created(c, gin.H{"id": 1}) })

	w := doRequest(t, r, http.MethodPost, "/res")
	require.Equal(t, http.StatusCreated, w.Code)
	b := decode(t, w)
	assert.Equal(t, http.StatusCreated, b.Code)
	assert.True(t, b.Success)
	assert.JSONEq(t, `{"id":1}`, string(b.Contents))
}

func TestNoContent(t *testing.T) {
	r := newTestEngine()
	r.DELETE("/res/1", func(c *gin.Context) { api.NoContent(c) })

	w := doRequest(t, r, http.MethodDelete, "/res/1")
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String(), "204 无响应体")
}

func TestOKPage(t *testing.T) {
	r := newTestEngine()
	r.GET("/list", func(c *gin.Context) {
		api.OKPage(c, paging.NewPageable[string](
			paging.Query{PageIndex: 2, PageSize: 2}, 5, []string{"a", "b"}))
	})

	b := decode(t, doRequest(t, r, http.MethodGet, "/list"))
	assert.Equal(t, http.StatusOK, b.Code)
	assert.True(t, b.Success)
	assert.JSONEq(t,
		`{"pageSize":2,"pageIndex":2,"totalSize":5,"data":["a","b"]}`, string(b.Contents))
}

func TestOKPageEmptyData(t *testing.T) {
	r := newTestEngine()
	r.GET("/empty", func(c *gin.Context) {
		api.OKPage(c, paging.NewPageable[int](paging.Query{}, 0, nil))
	})

	b := decode(t, doRequest(t, r, http.MethodGet, "/empty"))
	assert.JSONEq(t, `{"pageSize":20,"pageIndex":1,"totalSize":0,"data":[]}`, string(b.Contents),
		"空结果 data 必须是 [] 而非 null")
}

func TestFail(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantErrMsg string
	}{
		{
			name:       "已知错误码按码表映射",
			err:        errs.New(errs.CodeNotFound, "user 42 not found"),
			wantStatus: http.StatusNotFound,
			wantErrMsg: "user 42 not found",
		},
		{
			name:       "带明细不出参",
			err:        errs.New(errs.CodeValidation, "参数非法").WithDetail("field", "email"),
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "参数非法",
		},
		{
			name:       "包装错误沿用外层语义",
			err:        errs.Wrap(errors.New("dial timeout"), errs.CodeDB, "query orders"),
			wantStatus: http.StatusInternalServerError,
			wantErrMsg: "query orders",
		},
		{
			name:       "未映射自定义码按 500",
			err:        errs.New(errs.Code("USER_DISABLED"), "账号已禁用"),
			wantStatus: http.StatusInternalServerError,
			wantErrMsg: "账号已禁用",
		},
		{
			name:       "sql.ErrNoRows 转 404",
			err:        sql.ErrNoRows,
			wantStatus: http.StatusNotFound,
			wantErrMsg: "resource not found",
		},
		{
			name:       "context.Canceled 转 499",
			err:        context.Canceled,
			wantStatus: api.StatusClientClosedRequest,
			wantErrMsg: "client canceled request",
		},
		{
			name:       "未知错误根因不泄漏",
			err:        errors.New("secret token xyz"),
			wantStatus: http.StatusInternalServerError,
			wantErrMsg: "internal error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestEngine()
			r.GET("/f", func(c *gin.Context) { api.Fail(c, tt.err) })

			w := doRequest(t, r, http.MethodGet, "/f")
			require.Equal(t, tt.wantStatus, w.Code)

			b := decode(t, w)
			assert.Equal(t, tt.wantStatus, b.Code, "code 即 HTTP 状态码")
			assert.False(t, b.Success)
			assert.Equal(t, tt.wantErrMsg, b.ErrMsg)
			assert.Equal(t, "null", string(b.Contents), "失败时 contents 为 null")
			assert.NotContains(t, w.Body.String(), "details", "明细不得进入响应体")
			assert.NotContains(t, w.Body.String(), "secret token", "根因信息不得进入响应体")
		})
	}
}

func TestWriteSkipsWhenWritten(t *testing.T) {
	r := newTestEngine()
	r.GET("/w", func(c *gin.Context) {
		c.String(http.StatusOK, "first")
		r := api.NewResponse(c, http.StatusOK, "")
		api.Write(c, http.StatusOK, &r)
	})

	w := doRequest(t, r, http.MethodGet, "/w")
	assert.Equal(t, "first", w.Body.String(), "已写过的响应不得被覆盖")
}

func TestNoContentSkipsWhenWritten(t *testing.T) {
	r := newTestEngine()
	r.GET("/nc", func(c *gin.Context) {
		c.String(http.StatusOK, "first")
		api.NoContent(c)
	})

	w := doRequest(t, r, http.MethodGet, "/nc")
	assert.Equal(t, "first", w.Body.String())
}

func TestFailSkipsWhenWritten(t *testing.T) {
	r := newTestEngine()
	r.GET("/f", func(c *gin.Context) {
		c.String(http.StatusOK, "first")
		api.Fail(c, errs.New(errs.CodeNotFound, "ignored"))
	})

	w := doRequest(t, r, http.MethodGet, "/f")
	assert.Equal(t, "first", w.Body.String())
}

func TestStatusFor(t *testing.T) {
	assert.Equal(t, http.StatusOK, api.StatusFor(errs.CodeOK))
	assert.Equal(t, http.StatusNotFound, api.StatusFor(errs.CodeNotFound))
	assert.Equal(t, http.StatusBadGateway, api.StatusFor(errs.CodeUpstream))
	assert.Equal(t, http.StatusInternalServerError, api.StatusFor(errs.Code("CUSTOM_X")),
		"未映射码按 500")
}
