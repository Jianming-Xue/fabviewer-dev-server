package errresp_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/api/errresp"
	"github.com/fab-viewer/common-lib/api/internal/testutil"
	"github.com/fab-viewer/common-lib/common/bizerror"
)

func TestFail(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantErrMsg string
	}{
		{
			name:       "已知错误码按码表映射",
			err:        bizerror.NewError(bizerror.CodeNotFound, "user 42 not found"),
			wantStatus: http.StatusNotFound,
			wantErrMsg: "user 42 not found",
		},
		{
			name:       "带明细不出参",
			err:        bizerror.NewError(bizerror.CodeValidation, "参数非法").AttachError(nil, map[string]any{"field": "email"}),
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "参数非法",
		},
		{
			name:       "包装错误沿用外层语义",
			err:        bizerror.NewError(bizerror.CodeDB, "query orders").AttachError(errors.New("dial timeout"), nil),
			wantStatus: http.StatusInternalServerError,
			wantErrMsg: "query orders",
		},
		{
			name:       "未映射自定义码按 500",
			err:        bizerror.NewError(bizerror.Code("USER_DISABLED"), "账号已禁用"),
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
			wantStatus: errresp.StatusClientClosedRequest,
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
			r := testutil.NewTestEngine()
			r.GET("/f", func(c *gin.Context) { errresp.Fail(c, tt.err) })

			w := testutil.DoGet(r, "/f")
			require.Equal(t, tt.wantStatus, w.Code)

			b := testutil.Decode(t, w)
			assert.Equal(t, tt.wantStatus, b.Code, "code 即 HTTP 状态码")
			assert.False(t, b.Success)
			assert.Equal(t, tt.wantErrMsg, b.ErrMsg)
			assert.Equal(t, "null", string(b.Contents), "失败时 contents 为 null")
			assert.NotContains(t, w.Body.String(), "details", "明细不得进入响应体")
			assert.NotContains(t, w.Body.String(), "secret token", "根因信息不得进入响应体")
		})
	}
}

func TestFailSkipsWhenWritten(t *testing.T) {
	r := testutil.NewTestEngine()
	r.GET("/f", func(c *gin.Context) {
		c.String(http.StatusOK, "first")
		errresp.Fail(c, bizerror.NewError(bizerror.CodeNotFound, "ignored"))
	})

	w := testutil.DoGet(r, "/f")
	assert.Equal(t, "first", w.Body.String())
}

func TestStatusFor(t *testing.T) {
	assert.Equal(t, http.StatusOK, errresp.StatusFor(bizerror.CodeOK))
	assert.Equal(t, http.StatusNotFound, errresp.StatusFor(bizerror.CodeNotFound))
	assert.Equal(t, http.StatusBadGateway, errresp.StatusFor(bizerror.CodeUpstream))
	assert.Equal(t, http.StatusInternalServerError, errresp.StatusFor(bizerror.Code("CUSTOM_X")),
		"未映射码按 500")
}
