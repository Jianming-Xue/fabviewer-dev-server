package fault_test

import (
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
	"github.com/fab-viewer/common-lib/api/middleware/fault"
	"github.com/fab-viewer/common-lib/common/bizerror"
)

// faultBody 是错误响应的解码目标（response.ApiResponse）。
type faultBody struct {
	Code    int    `json:"code"`
	Success bool   `json:"success"`
	ErrMsg  string `json:"errMsg"`
	Data    any    `json:"data"`
	TxID    string `json:"txId"`
}

func TestFaultErrMsgIsRawError(t *testing.T) {
	// 出口不做任何翻译或固定文案替换：errMsg 直取 err.Error()，
	// 未知错误的根因也直出响应体（业务方拍板取舍）。
	tests := []struct {
		name       string
		err        error
		wantCode   int
		wantErrMsg string
	}{
		{"BizError 未映射 code 0", bizerror.NewError(bizerror.CodeNotFound, "user not found"), 0, "user not found"},
		{"BizError 带 Details 只出 Message", bizerror.NewError(bizerror.CodeValidation, "参数非法").
			AttachError(nil, map[string]any{"field": "email"}), 0, "参数非法"},
		{"gorm not found 不再翻译", gorm.ErrRecordNotFound, 0, gorm.ErrRecordNotFound.Error()},
		{"wrapped 错误", fmt.Errorf("load: %w", gorm.ErrRecordNotFound), 0, "load: " + gorm.ErrRecordNotFound.Error()},
		{"未知错误根因直出", errors.New("dial tcp: password=hunter2 refused"), 0, "dial tcp: password=hunter2 refused"},
		{"context canceled 不再特判", contextCanceled(), 0, "context canceled"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := testutil.CaptureLog(t)
			r := testutil.NewTestEngine()
			r.Use(fault.Fault())
			r.GET("/e", func(c *gin.Context) { _ = c.Error(tt.err) })

			w := testutil.DoGet(r, "/e")
			require.Equal(t, http.StatusOK, w.Code, "HTTP 恒 200")

			var b faultBody
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
			assert.Equal(t, tt.wantCode, b.Code)
			assert.False(t, b.Success)
			assert.Nil(t, b.Data)
			assert.Equal(t, tt.wantErrMsg, b.ErrMsg, "errMsg 直取错误消息，出口零翻译")

			// 失败日志：Error 级、含错误原文与调用栈
			records := log.Snapshot()
			require.Len(t, records, 1)
			assert.Equal(t, "error", records[0].Level)
			assert.Equal(t, "request fault", records[0].Msg)
			v, ok := testutil.KVGet(records[0].KV, "err")
			require.True(t, ok)
			assert.Equal(t, tt.err.Error(), v)
			_, ok = testutil.KVGet(records[0].KV, "stack")
			assert.True(t, ok, "失败日志必须带调用栈")
		})
	}
}

func TestFaultBizCodeAndDetailsToLog(t *testing.T) {
	log := testutil.CaptureLog(t)
	r := testutil.NewTestEngine()
	r.Use(fault.Fault())
	r.GET("/e", func(c *gin.Context) {
		_ = c.Error(bizerror.NewError(bizerror.CodeValidation, "参数非法").
			AttachError(errors.New("root"), map[string]any{"field": "email"}))
	})

	w := testutil.DoGet(r, "/e")
	var b faultBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
	assert.Equal(t, "参数非法", b.ErrMsg)
	assert.NotContains(t, w.Body.String(), "email", "Details 不出响应体")
	assert.NotContains(t, w.Body.String(), "VALIDATION_ERROR", "内部码格式不出响应体")

	records := log.Snapshot()
	require.Len(t, records, 1)
	code, _ := testutil.KVGet(records[0].KV, "bizCode")
	assert.Equal(t, string(bizerror.CodeValidation), code, "字符串业务码进日志")
	details, _ := testutil.KVGet(records[0].KV, "details")
	assert.Contains(t, details, "email", "Details 进日志留痕")
}

func TestFaultWithCodeMap(t *testing.T) {
	testutil.CaptureLog(t)
	r := testutil.NewTestEngine()
	r.Use(fault.Fault(fault.WithCodeMap(map[bizerror.Code]int{
		bizerror.Code("USER_DISABLED"): 1003,
	})))
	r.GET("/e", func(c *gin.Context) {
		_ = c.Error(bizerror.NewError(bizerror.Code("USER_DISABLED"), "禁用"))
	})

	var b faultBody
	require.NoError(t, json.Unmarshal(testutil.DoGet(r, "/e").Body.Bytes(), &b))
	assert.Equal(t, 1003, b.Code, "模块码经 WithCodeMap 映射业务 code")
}

func TestFaultMultipleErrorsUsesLast(t *testing.T) {
	testutil.CaptureLog(t)
	r := testutil.NewTestEngine()
	r.Use(fault.Fault())
	r.GET("/e", func(c *gin.Context) {
		_ = c.Error(bizerror.NewError(bizerror.CodeInternal, "first"))
		_ = c.Error(bizerror.NewError(bizerror.CodeNotFound, "second"))
	})

	var b faultBody
	require.NoError(t, json.Unmarshal(testutil.DoGet(r, "/e").Body.Bytes(), &b))
	assert.Equal(t, "second", b.ErrMsg, "多个错误取最后一个")
}

func TestFaultSkipsWhenWritten(t *testing.T) {
	testutil.CaptureLog(t)
	r := testutil.NewTestEngine()
	r.Use(fault.Fault())
	r.GET("/e", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"written": true})
		_ = c.Error(bizerror.NewError(bizerror.CodeNotFound, "ignored"))
	})

	w := testutil.DoGet(r, "/e")
	assert.JSONEq(t, `{"written":true}`, w.Body.String(), "已写过响应不覆盖")
}

func TestFaultNoErrors(t *testing.T) {
	log := testutil.CaptureLog(t)
	r := testutil.NewTestEngine()
	r.Use(fault.Fault())
	r.GET("/ok", func(c *gin.Context) { c.String(http.StatusOK, "fine") })

	w := testutil.DoGet(r, "/ok")
	assert.Equal(t, "fine", w.Body.String())
	assert.Empty(t, log.Snapshot(), "无错误不产生失败日志")
}

// contextCanceled 避免 import context 到表驱动。
func contextCanceled() error { return errors.New("context canceled") }
