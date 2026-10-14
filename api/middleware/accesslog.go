package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/common-lib/api"
	"github.com/fab-viewer/common-lib/logx"
)

type accessLogConfig struct {
	skipPaths map[string]struct{}
}

// AccessLogOption 配置 AccessLog 中间件。
type AccessLogOption func(*accessLogConfig)

// WithSkipPaths 追加不记录访问日志的路径（默认已含 /healthz、/livez、/readyz）。
func WithSkipPaths(paths ...string) AccessLogOption {
	return func(cfg *accessLogConfig) {
		for _, p := range paths {
			cfg.skipPaths[p] = struct{}{}
		}
	}
}

// AccessLog 记录访问日志：method/path/status/latency_ms/client_ip/
// request_id/bytes。log 为 nil 时使用 logx.Default()。
func AccessLog(log logx.Logger, opts ...AccessLogOption) gin.HandlerFunc {
	cfg := accessLogConfig{skipPaths: map[string]struct{}{
		"/healthz": {},
		"/livez":   {},
		"/readyz":  {},
	}}
	for _, opt := range opts {
		opt(&cfg)
	}
	if log == nil {
		log = logx.Default()
	}
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		if _, skip := cfg.skipPaths[c.Request.URL.Path]; skip {
			return
		}
		log.Info("access",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
			"request_id", api.RequestIDOf(c),
			"bytes", c.Writer.Size(),
		)
	}
}
