package accesslog

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/common-lib/api/reqctx"
	"github.com/fab-viewer/common-lib/common/logx"
)

type accessLogConfig struct {
	skipPaths map[string]struct{}
}

// Option 配置 AccessLog 中间件。
type Option func(*accessLogConfig)

// WithSkipPaths 追加不记录访问日志的路径（默认已含 /healthz、/livez、/readyz）。
func WithSkipPaths(paths ...string) Option {
	return func(cfg *accessLogConfig) {
		for _, p := range paths {
			cfg.skipPaths[p] = struct{}{}
		}
	}
}

// AccessLog 记录访问日志：method/path/status/latency_ms/client_ip/
// tx_id/tx_name/user/bytes。经全局日志出口 logx 记录（Init 落位；测试经
// logx.SetLogger 注入），组件零持有日志对象，无 Init 时序问题。
func AccessLog(opts ...Option) gin.HandlerFunc {
	cfg := accessLogConfig{skipPaths: map[string]struct{}{
		"/healthz": {},
		"/livez":   {},
		"/readyz":  {},
	}}
	for _, opt := range opts {
		opt(&cfg)
	}
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		if _, skip := cfg.skipPaths[c.Request.URL.Path]; skip {
			return
		}
		rc := reqctx.RuntimeContextOf(c)
		logx.Info("access",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
			"tx_id", rc.FromID(),
			"tx_name", rc.FromName(),
			"user", rc.FromUser(),
			"bytes", c.Writer.Size(),
		)
	}
}
