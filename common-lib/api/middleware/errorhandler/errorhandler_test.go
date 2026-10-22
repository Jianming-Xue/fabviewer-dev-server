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
	"github.com/fab-viewer/common-lib/api/middleware/enrich"
	"github.com/fab-viewer/common-lib/api/middleware/errorhandler"
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
	// HTTP 恒 200；业务 code 进响应体：未映射码（含内置码）一律 0，
	// gorm/sql not found 与 canceled 有固定文案
	tests := []struct {
		name string
		err  error
	}{
		{"内置 validation 码未映射按 0", bizerror.NewError(bizerror.CodeValidation, "email 格式不正确")},
		{"内置 not found 码未映射按 0", bizerror.NewError(bizerror.CodeNotFound, "user not found")},
		{"db 根因只进日志", bizerror.NewError(bizerror.CodeDB, "query orders").AttachError(errors.New("dial tcp: refused"), nil)},
		{"gorm not found", gorm.ErrRecordNotFound},
		{"wrapped gorm not found", fmt.Errorf("load: %w", gorm.ErrRecordNotFound)},
		// BizError 按自身 Code 查码表（errors.As 先拦截，根因不参与）：
		// 业务要特定业务码应自身携带模块码并经 WithCodeMap 映射
		{"bizerror carrying gorm not found as root",
			bizerror.NewError(bizerror.CodeDB, "repository.Create").AttachError(gorm.ErrRecordNotFound, nil)},
		{"sql no rows", sql.ErrNoRows},
		{"module code unmapped", bizerror.NewError(bizerror.Code("USER_DISABLED"), "账号已禁用")},
		{"context canceled", context.Canceled},
		{"unknown error", errors.New("boom")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutil.CaptureLog(t)
			r := testutil.NewTestEngine()
			r.Use(errorhandler.ErrorHandler())
			r.GET("/e", func(c *gin.Context) { _ = c.Error(tt.err) })

			w := testutil.DoGet(r, "/e")
			require.Equal(t, http.StatusOK, w.Code, "HTTP 状态码恒 200")

			var b errBody
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
			assert.Equal(t, 0, b.Code, "未映射码按 Fail 默认 0")
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
		enrich.Enrich(),
	)
	r.GET("/e", func(c *gin.Context) {
		_ = c.Error(bizerror.NewError(bizerror.CodeNotFound, "用户 42 不存在").AttachError(errors.New("root"), nil))
	})

	w := testutil.DoGet(r, "/e")
	var b errBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
	assert.Equal(t, "用户 42 不存在", b.ErrMsg, "Message 直出，出口不翻译")
	assert.Equal(t, 0, b.Code, "未映射码按默认 0")

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
	require.Equal(t, http.StatusOK, w.Code)

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
	require.Equal(t, http.StatusOK, w.Code)

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
	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "hunter2", "根因不进响应")

	records := log.Snapshot()
	require.Len(t, records, 1)
	assert.Equal(t, "error", records[0].Level)
	v, ok := testutil.KVGet(records[0].KV, "err")
	require.True(t, ok)
	assert.Contains(t, v, "hunter2", "根因进日志")
}

func TestErrorHandlerWithCodeMap(t *testing.T) {
	testutil.CaptureLog(t)
	r := testutil.NewTestEngine()
	r.Use(errorhandler.ErrorHandler(errorhandler.WithCodeMap(map[bizerror.Code]int{
		bizerror.Code("USER_DISABLED"): 1003,
	})))
	r.GET("/e", func(c *gin.Context) { _ = c.Error(bizerror.NewError(bizerror.Code("USER_DISABLED"), "禁用")) })

	w := testutil.DoGet(r, "/e")
	var b errBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1003, b.Code, "模块码经 WithCodeMap 映射业务 code")
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
	assert.Equal(t, 0, b.Code, "多个错误取最后一个")
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
