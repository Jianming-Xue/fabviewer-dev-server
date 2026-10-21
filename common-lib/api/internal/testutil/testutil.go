// Package testutil 提供 common-lib API 子树共享的测试脚手架：日志捕获、
// 测试引擎、请求发起与 kv 断言。位于 api/internal 下，仅本模块的
// api/... 测试可 import，业务服务不可见。
package testutil

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/common/logx"
)

// CapturedRecord 记录一次日志调用。
type CapturedRecord struct {
	Level string // debug|info|warn|error
	Msg   string
	KV    []any
}

// CaptureLogger 是收集全部日志调用的 logx.Logger 假实现（并发安全）。
type CaptureLogger struct {
	mu      sync.Mutex
	records []CapturedRecord
}

// Debug 记录 debug 级别（仅收集，不输出）。
func (l *CaptureLogger) Debug(msg string, kv ...any) { l.add("debug", msg, kv) }

// Info 记录 info 级别（仅收集，不输出）。
func (l *CaptureLogger) Info(msg string, kv ...any) { l.add("info", msg, kv) }

// Warn 记录 warn 级别（仅收集，不输出）。
func (l *CaptureLogger) Warn(msg string, kv ...any) { l.add("warn", msg, kv) }

// Error 记录 error 级别（仅收集，不输出）。
func (l *CaptureLogger) Error(msg string, kv ...any) { l.add("error", msg, kv) }

// With 返回自身（派生字段不区分，统一收集）。
func (l *CaptureLogger) With(_ ...any) logx.Logger { return l }

func (l *CaptureLogger) add(level, msg string, kv []any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	kvCopy := make([]any, len(kv))
	copy(kvCopy, kv)
	l.records = append(l.records, CapturedRecord{Level: level, Msg: msg, KV: kvCopy})
}

// Snapshot 返回迄今收集的全部日志记录副本。
func (l *CaptureLogger) Snapshot() []CapturedRecord {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]CapturedRecord, len(l.records))
	copy(out, l.records)
	return out
}

// CaptureLog 注入 CaptureLogger 为全局日志出口（logx.SetLogger），并注册
// 测试结束恢复为 Nop，返回该捕获器。用于验证经全局出口记录日志的组件
// （零持有日志对象的中间件等）。
func CaptureLog(t *testing.T) *CaptureLogger {
	t.Helper()
	l := &CaptureLogger{}
	logx.SetLogger(l)
	t.Cleanup(func() { logx.SetLogger(logx.Nop()) })
	return l
}

// NewTestEngine 返回处于 TestMode 的裸 gin 引擎。
func NewTestEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

// Do 发起指定方法的请求并返回响应记录器。
func Do(r http.Handler, method, target string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), method, target, http.NoBody)
	r.ServeHTTP(w, req)
	return w
}

// DoGet 发起 GET 请求并返回响应记录器。
func DoGet(r http.Handler, target string) *httptest.ResponseRecorder {
	return Do(r, http.MethodGet, target)
}

// DoGetWithHeaders 发起携带多个请求头的 GET 请求。
func DoGetWithHeaders(r http.Handler, target string, headers map[string]string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, target, http.NoBody)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	r.ServeHTTP(w, req)
	return w
}

// KVGet 从 kv 变长对中按键取值（对不存在返回 false）。
func KVGet(kv []any, key string) (any, bool) {
	for i := 0; i+1 < len(kv); i += 2 {
		if k, ok := kv[i].(string); ok && k == key {
			return kv[i+1], true
		}
	}
	return nil, false
}

// MustKVString 从 kv 变长对中按键取字符串值，缺失或类型不符时测试失败。
func MustKVString(t *testing.T, kv []any, key string) string {
	t.Helper()
	v, ok := KVGet(kv, key)
	require.True(t, ok, "缺少字段 %s", key)
	s, ok := v.(string)
	require.True(t, ok, "字段 %s 应为字符串", key)
	return s
}

// RespBody 是统一响应体的测试解码目标（与 response.ApiResponse 字段一一对应）。
type RespBody struct {
	Code    int             `json:"code"`
	Success bool            `json:"success"`
	ErrMsg  string          `json:"errMsg"`
	Data    json.RawMessage `json:"data"`
	TxID    string          `json:"txId"`
}

// Decode 将响应体 JSON 解码为 RespBody。
func Decode(t *testing.T, w *httptest.ResponseRecorder) RespBody {
	t.Helper()
	var b RespBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &b))
	return b
}
