package middleware_test

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

	"github.com/fab-viewer/common-lib/api/middleware"
	"github.com/fab-viewer/common-lib/common/errs"
	"github.com/fab-viewer/common-lib/common/logx"
)

// errBody 是错误响应的解码目标（api.Response）。
type errBody struct {
	Code     int    `json:"code"`
	Success  bool   `json:"success"`
	ErrMsg   string `json:"errMsg"`
	Contents any    `json:"contents"`
	TxID     string `json:"txId"`
}

func TestErrorHandlerCodeMap(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"validation", errs.New(errs.CodeValidation, "email 格式不正确"), http.StatusBadRequest},
		{"unauthorized", errs.New(errs.CodeUnauthorized, "未登录"), http.StatusUnauthorized},
		{"forbidden", errs.New(errs.CodeForbidden, "无权限"), http.StatusForbidden},
		{"not found", errs.New(errs.CodeNotFound, "user not found"), http.StatusNotFound},
		{"conflict", errs.New(errs.CodeConflict, "已存在"), http.StatusConflict},
		{"too many", errs.New(errs.CodeTooMany, "限流"), http.StatusTooManyRequests},
		{"internal", errs.New(errs.CodeInternal, "内部错误"), http.StatusInternalServerError},
		{"db", errs.Wrap(errors.New("dial tcp: refused"), errs.CodeDB, "query orders"), http.StatusInternalServerError},
		{"upstream", errs.New(errs.CodeUpstream, "下游不可用"), http.StatusBadGateway},
		{"gorm not found", gorm.ErrRecordNotFound, http.StatusNotFound},
		{"wrapped gorm not found", fmt.Errorf("load: %w", gorm.ErrRecordNotFound), http.StatusNotFound},
		{"sql no rows", sql.ErrNoRows, http.StatusNotFound},
		{"custom code unmapped", errs.New(errs.Code("USER_DISABLED"), "账号已禁用"), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := &captureLogger{}
			r := newTestEngine()
			r.Use(middleware.ErrorHandler(log))
			r.GET("/e", func(c *gin.Context) { _ = c.Error(tt.err) })

			w := doGet(r, "/e")
			require.Equal(t, tt.wantStatus, w.Code)

			var b errBody
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
			assert.Equal(t, tt.wantStatus, b.Code, "code 即 HTTP 状态码")
			assert.False(t, b.Success)
			assert.Nil(t, b.Contents)
		})
	}
}

func TestErrorHandlerDetailsToLogNotResponse(t *testing.T) {
	log := &captureLogger{}
	r := newTestEngine()
	r.Use(middleware.ErrorHandler(log))
	r.GET("/e", func(c *gin.Context) {
		_ = c.Error(errs.New(errs.CodeValidation, "参数非法").WithDetail("field", "email"))
	})

	w := doGet(r, "/e")
	require.Equal(t, http.StatusBadRequest, w.Code)

	var b errBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
	assert.Equal(t, "参数非法", b.ErrMsg)
	assert.NotContains(t, w.Body.String(), "details", "明细不出参")
	assert.NotContains(t, w.Body.String(), "email", "明细值不出参")

	records := log.snapshot()
	require.Len(t, records, 1)
	assert.Equal(t, "info", records[0].Level, "业务错误按 info 记录")
	assert.Equal(t, "business error", records[0].Msg)
	code, _ := kvGet(records[0].KV, "code")
	assert.Equal(t, string(errs.CodeValidation), code, "字符串错误码进日志")
	details, _ := kvGet(records[0].KV, "details")
	assert.Contains(t, details, "email", "明细进日志")
}

func TestErrorHandlerContextCanceled(t *testing.T) {
	log := &captureLogger{}
	r := newTestEngine()
	r.Use(middleware.ErrorHandler(log))
	r.GET("/e", func(c *gin.Context) { _ = c.Error(context.Canceled) })

	w := doGet(r, "/e")
	require.Equal(t, 499, w.Code)

	records := log.snapshot()
	require.Len(t, records, 1)
	assert.Equal(t, "info", records[0].Level, "客户端断开按 info 记录")
	assert.Equal(t, "client canceled request", records[0].Msg)
}

func TestErrorHandlerUnknownError(t *testing.T) {
	log := &captureLogger{}
	r := newTestEngine()
	r.Use(middleware.ErrorHandler(log))
	r.GET("/e", func(c *gin.Context) { _ = c.Error(errors.New("password is hunter2")) })

	w := doGet(r, "/e")
	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.NotContains(t, w.Body.String(), "hunter2", "根因不进响应")

	records := log.snapshot()
	require.Len(t, records, 1)
	assert.Equal(t, "error", records[0].Level)
	v, ok := kvGet(records[0].KV, "err")
	require.True(t, ok)
	assert.Contains(t, v, "hunter2", "根因进日志")
}

func TestErrorHandlerWithStatusMap(t *testing.T) {
	log := &captureLogger{}
	r := newTestEngine()
	r.Use(middleware.ErrorHandler(log, middleware.WithStatusMap(map[errs.Code]int{
		errs.Code("USER_DISABLED"): http.StatusForbidden,
	})))
	r.GET("/e", func(c *gin.Context) { _ = c.Error(errs.New(errs.Code("USER_DISABLED"), "禁用")) })

	w := doGet(r, "/e")
	assert.Equal(t, http.StatusForbidden, w.Code, "自定义码经 WithStatusMap 映射")
}

func TestErrorHandlerWithMessageFunc(t *testing.T) {
	log := &captureLogger{}
	r := newTestEngine()
	r.Use(middleware.ErrorHandler(log, middleware.WithMessageFunc(func(_ *errs.Error) string {
		return "i18n: not found"
	})))
	r.GET("/e", func(c *gin.Context) { _ = c.Error(errs.New(errs.CodeNotFound, "raw message")) })
	r.GET("/keep", func(c *gin.Context) {
		_ = c.Error(errs.New(errs.CodeNotFound, "kept message"))
	})

	var b errBody
	require.NoError(t, json.Unmarshal(doGet(r, "/e").Body.Bytes(), &b))
	assert.Equal(t, "i18n: not found", b.ErrMsg)

	// messageFn 返回空串时保留原 errMsg
	r2 := newTestEngine()
	r2.Use(middleware.ErrorHandler(log, middleware.WithMessageFunc(func(_ *errs.Error) string {
		return ""
	})))
	r2.GET("/e", func(c *gin.Context) { _ = c.Error(errs.New(errs.CodeNotFound, "kept message")) })
	require.NoError(t, json.Unmarshal(doGet(r2, "/e").Body.Bytes(), &b))
	assert.Equal(t, "kept message", b.ErrMsg)
}

func TestErrorHandlerMultipleErrorsUsesLast(t *testing.T) {
	log := &captureLogger{}
	r := newTestEngine()
	r.Use(middleware.ErrorHandler(log))
	r.GET("/e", func(c *gin.Context) {
		_ = c.Error(errs.New(errs.CodeInternal, "first"))
		_ = c.Error(errs.New(errs.CodeNotFound, "second"))
	})

	var b errBody
	require.NoError(t, json.Unmarshal(doGet(r, "/e").Body.Bytes(), &b))
	assert.Equal(t, http.StatusNotFound, b.Code, "多个错误取最后一个")
	assert.Equal(t, "second", b.ErrMsg)
}

func TestErrorHandlerSkipsWhenWritten(t *testing.T) {
	log := &captureLogger{}
	r := newTestEngine()
	r.Use(middleware.ErrorHandler(log))
	r.GET("/e", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"written": true})
		_ = c.Error(errs.New(errs.CodeNotFound, "ignored"))
	})

	w := doGet(r, "/e")
	assert.JSONEq(t, `{"written":true}`, w.Body.String(), "已写过响应不覆盖")
}

func TestErrorHandlerNoErrors(t *testing.T) {
	log := &captureLogger{}
	r := newTestEngine()
	r.Use(middleware.ErrorHandler(log))
	r.GET("/ok", func(c *gin.Context) { c.String(http.StatusOK, "fine") })

	w := doGet(r, "/ok")
	assert.Equal(t, "fine", w.Body.String())
	assert.Empty(t, log.snapshot())
}

func TestErrorHandlerNilLoggerUsesDefault(t *testing.T) {
	capture := &captureLogger{}
	logx.SetDefault(capture)
	t.Cleanup(func() { logx.SetDefault(logx.Nop()) })

	r := newTestEngine()
	r.Use(middleware.ErrorHandler(nil))
	r.GET("/e", func(c *gin.Context) { _ = c.Error(errors.New("boom")) })

	doGet(r, "/e")
	assert.Len(t, capture.snapshot(), 1, "nil logger 应回退到 logx.Default()")
}
