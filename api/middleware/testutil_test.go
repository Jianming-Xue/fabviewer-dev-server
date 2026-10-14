package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/common-lib/logx"
)

// capturedRecord 记录一次日志调用。
type capturedRecord struct {
	Level string // debug|info|warn|error
	Msg   string
	KV    []any
}

// captureLogger 是收集全部日志调用的 logx.Logger 假实现（并发安全）。
type captureLogger struct {
	mu      sync.Mutex
	records []capturedRecord
}

func (l *captureLogger) Debug(msg string, kv ...any) { l.add("debug", msg, kv) }
func (l *captureLogger) Info(msg string, kv ...any)  { l.add("info", msg, kv) }
func (l *captureLogger) Warn(msg string, kv ...any)  { l.add("warn", msg, kv) }
func (l *captureLogger) Error(msg string, kv ...any) { l.add("error", msg, kv) }
func (l *captureLogger) With(_ ...any) logx.Logger   { return l }

func (l *captureLogger) add(level, msg string, kv []any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	kvCopy := make([]any, len(kv))
	copy(kvCopy, kv)
	l.records = append(l.records, capturedRecord{Level: level, Msg: msg, KV: kvCopy})
}

func (l *captureLogger) snapshot() []capturedRecord {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]capturedRecord, len(l.records))
	copy(out, l.records)
	return out
}

// kvGet 从 kv 变长对中按键取值（对不存在返回 false）。
func kvGet(kv []any, key string) (any, bool) {
	for i := 0; i+1 < len(kv); i += 2 {
		if k, ok := kv[i].(string); ok && k == key {
			return kv[i+1], true
		}
	}
	return nil, false
}

func newTestEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func doGet(r http.Handler, target string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, target, http.NoBody)
	r.ServeHTTP(w, req)
	return w
}

func doRequestWithHeader(
	r http.Handler, method, target, header, value string,
) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), method, target, http.NoBody)
	req.Header.Set(header, value)
	r.ServeHTTP(w, req)
	return w
}
