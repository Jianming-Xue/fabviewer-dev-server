package errorhandler_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/fab-viewer/common-lib/api/internal/testutil"
	"github.com/fab-viewer/common-lib/api/middleware/errorhandler"
	"github.com/fab-viewer/common-lib/api/middleware/runtimecontext"
	"github.com/fab-viewer/common-lib/common/bizerror"
)

// errBody 是错误响应的解码目标（response.Response）。
type errBody struct {
	Code    int    `json:"code"`
	Success bool   `json:"success"`
	ErrMsg  string `json:"errMsg"`
	Data    any    `json:"data"`
	TxID    string `json:"txId"`
}

func TestErrorHandlerCodeMap(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"validation", bizerror.NewError(bizerror.CodeValidation, "email 格式不正确"), http.StatusBadRequest},
		{"unauthorized", bizerror.NewError(bizerror.CodeUnauthorized, "未登录"), http.StatusUnauthorized},
		{"forbidden", bizerror.NewError(bizerror.CodeForbidden, "无权限"), http.StatusForbidden},
		{"not found", bizerror.NewError(bizerror.CodeNotFound, "user not found"), http.StatusNotFound},
		{"conflict", bizerror.NewError(bizerror.CodeConflict, "已存在"), http.StatusConflict},
		{"too many", bizerror.NewError(bizerror.CodeTooMany, "限流"), http.StatusTooManyRequests},
		{"internal", bizerror.NewError(bizerror.CodeInternal, "内部错误"), http.StatusInternalServerError},
		{"db", bizerror.NewError(bizerror.CodeDB, "query orders").AttachError(errors.New("dial tcp: refused"), nil), http.StatusInternalServerError},
		{"upstream", bizerror.NewError(bizerror.CodeUpstream, "下游不可用"), http.StatusBadGateway},
		{"gorm not found", gorm.ErrRecordNotFound, http.StatusNotFound},
		{"wrapped gorm not found", fmt.Errorf("load: %w", gorm.ErrRecordNotFound), http.StatusNotFound},
		// BizError 按自身 Code 映射（出口 errors.As 先拦截，根因不参与状态码）：
		// 业务要 NOT_FOUND 语义应自身携带 CodeNotFound，而非靠根因
		{"bizerror carrying gorm not found as root",
			bizerror.NewError(bizerror.CodeDB, "repository.Create").AttachError(gorm.ErrRecordNotFound, nil),
			http.StatusInternalServerError},
		{"sql no rows", sql.ErrNoRows, http.StatusNotFound},
		{"module code unmapped", bizerror.NewError(bizerror.Code("USER_DISABLED"), "账号已禁用"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutil.CaptureLog(t)
			r := testutil.NewTestEngine()
			r.Use(errorhandler.ErrorHandler())
			r.GET("/e", func(c *gin.Context) { _ = c.Error(tt.err) })

			w := testutil.DoGet(r, "/e")
			require.Equal(t, tt.wantStatus, w.Code)

			var b errBody
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
			assert.Equal(t, tt.wantStatus, b.Code, "code 即 HTTP 状态码")
			assert.False(t, b.Success)
			assert.Nil(t, b.Data)
		})
	}
}

func TestErrorHandlerFvMessageDirectOutput(t *testing.T) {
	// BizError 的 Message 创建时已翻译（bizerror.New 查模块字典 + InsertArgs），
	// 出口直接输出，不做翻译。
	log := testutil.CaptureLog(t)
	r := testutil.NewTestEngine()
	r.Use(
		errorhandler.ErrorHandler(),
		runtimecontext.FillRuntimeContext(),
	)
	r.GET("/e", func(c *gin.Context) {
		_ = c.Error(bizerror.NewError(bizerror.CodeNotFound, "用户 42 不存在").AttachError(errors.New("root"), nil))
	})

	w := testutil.DoGet(r, "/e")
	var b errBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
	assert.Equal(t, "用户 42 不存在", b.ErrMsg, "Message 直出，出口不翻译")
	assert.Equal(t, http.StatusNotFound, b.Code)

	records := log.Snapshot()
	require.Len(t, records, 1)
	code, _ := testutil.KVGet(records[0].KV, "code")
	assert.Equal(t, "NOT_FOUND", code, "错误码进日志")
}

func TestErrorHandlerDetailsToLogNotResponse(t *testing.T) {
	log := testutil.CaptureLog(t)
	r := testutil.NewTestEngine()
	r.Use(errorhandler.ErrorHandler())
	r.GET("/e", func(c *gin.Context) {
		_ = c.Error(bizerror.NewError(bizerror.CodeValidation, "参数非法").AttachError(nil, map[string]any{"field": "email"}))
	})

	w := testutil.DoGet(r, "/e")
	require.Equal(t, http.StatusBadRequest, w.Code)

	var b errBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
	assert.Equal(t, "参数非法", b.ErrMsg)
	assert.NotContains(t, w.Body.String(), "details", "明细不出参")
	assert.NotContains(t, w.Body.String(), "email", "明细值不出参")

	records := log.Snapshot()
	require.Len(t, records, 1)
	assert.Equal(t, "info", records[0].Level, "业务错误按 info 记录")
	assert.Equal(t, "business error", records[0].Msg)
	code, _ := testutil.KVGet(records[0].KV, "code")
	assert.Equal(t, string(bizerror.CodeValidation), code, "字符串错误码进日志")
	details, _ := testutil.KVGet(records[0].KV, "details")
	assert.Contains(t, details, "email", "明细进日志")
}

func TestErrorHandlerContextCanceled(t *testing.T) {
	log := testutil.CaptureLog(t)
	r := testutil.NewTestEngine()
	r.Use(errorhandler.ErrorHandler())
	r.GET("/e", func(c *gin.Context) { _ = c.Error(context.Canceled) })

	w := testutil.DoGet(r, "/e")
	require.Equal(t, 499, w.Code)

	records := log.Snapshot()
	require.Len(t, records, 1)
	assert.Equal(t, "info", records[0].Level, "客户端断开按 info 记录")
	assert.Equal(t, "client canceled request", records[0].Msg)
}

func TestErrorHandlerUnknownError(t *testing.T) {
	log := testutil.CaptureLog(t)
	r := testutil.NewTestEngine()
	r.Use(errorhandler.ErrorHandler())
	r.GET("/e", func(c *gin.Context) { _ = c.Error(errors.New("password is hunter2")) })

	w := testutil.DoGet(r, "/e")
	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.NotContains(t, w.Body.String(), "hunter2", "根因不进响应")

	records := log.Snapshot()
	require.Len(t, records, 1)
	assert.Equal(t, "error", records[0].Level)
	v, ok := testutil.KVGet(records[0].KV, "err")
	require.True(t, ok)
	assert.Contains(t, v, "hunter2", "根因进日志")
}

func TestErrorHandlerWithStatusMap(t *testing.T) {
	testutil.CaptureLog(t)
	r := testutil.NewTestEngine()
	r.Use(errorhandler.ErrorHandler(errorhandler.WithStatusMap(map[bizerror.Code]int{
		bizerror.Code("USER_DISABLED"): http.StatusForbidden,
	})))
	r.GET("/e", func(c *gin.Context) { _ = c.Error(bizerror.NewError(bizerror.Code("USER_DISABLED"), "禁用")) })

	w := testutil.DoGet(r, "/e")
	assert.Equal(t, http.StatusForbidden, w.Code, "模块码经 WithStatusMap 映射")
}

func TestErrorHandlerWithMessageFunc(t *testing.T) {
	testutil.CaptureLog(t)
	r := testutil.NewTestEngine()
	r.Use(errorhandler.ErrorHandler(errorhandler.WithMessageFunc(func(_ *gin.Context, _ *bizerror.BizError) string {
		return "overridden"
	})))
	r.GET("/e", func(c *gin.Context) { _ = c.Error(bizerror.NewError(bizerror.CodeNotFound, "raw message")) })

	var b errBody
	require.NoError(t, json.Unmarshal(testutil.DoGet(r, "/e").Body.Bytes(), &b))
	assert.Equal(t, "overridden", b.ErrMsg)

	// messageFn 返回空串时保留原 errMsg
	r2 := testutil.NewTestEngine()
	r2.Use(errorhandler.ErrorHandler(errorhandler.WithMessageFunc(func(_ *gin.Context, _ *bizerror.BizError) string {
		return ""
	})))
	r2.GET("/e", func(c *gin.Context) { _ = c.Error(bizerror.NewError(bizerror.CodeNotFound, "kept message")) })
	require.NoError(t, json.Unmarshal(testutil.DoGet(r2, "/e").Body.Bytes(), &b))
	assert.Equal(t, "kept message", b.ErrMsg)
}

func TestErrorHandlerMultipleErrorsUsesLast(t *testing.T) {
	testutil.CaptureLog(t)
	r := testutil.NewTestEngine()
	r.Use(errorhandler.ErrorHandler())
	r.GET("/e", func(c *gin.Context) {
		_ = c.Error(bizerror.NewError(bizerror.CodeInternal, "first"))
		_ = c.Error(bizerror.NewError(bizerror.CodeNotFound, "second"))
	})

	var b errBody
	require.NoError(t, json.Unmarshal(testutil.DoGet(r, "/e").Body.Bytes(), &b))
	assert.Equal(t, http.StatusNotFound, b.Code, "多个错误取最后一个")
	assert.Equal(t, "second", b.ErrMsg)
}

func TestErrorHandlerSkipsWhenWritten(t *testing.T) {
	testutil.CaptureLog(t)
	r := testutil.NewTestEngine()
	r.Use(errorhandler.ErrorHandler())
	r.GET("/e", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"written": true})
		_ = c.Error(bizerror.NewError(bizerror.CodeNotFound, "ignored"))
	})

	w := testutil.DoGet(r, "/e")
	assert.JSONEq(t, `{"written":true}`, w.Body.String(), "已写过响应不覆盖")
}

func TestErrorHandlerNoErrors(t *testing.T) {
	log := testutil.CaptureLog(t)
	r := testutil.NewTestEngine()
	r.Use(errorhandler.ErrorHandler())
	r.GET("/ok", func(c *gin.Context) { c.String(http.StatusOK, "fine") })

	w := testutil.DoGet(r, "/ok")
	assert.Equal(t, "fine", w.Body.String())
	assert.Empty(t, log.Snapshot())
}
