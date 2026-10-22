package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/common-lib/api/middleware/enrich"
	"github.com/fab-viewer/common-lib/api/middleware/errorhandler"
	"github.com/fab-viewer/common-lib/api/middleware/guard"
	"github.com/fab-viewer/common-lib/api/middleware/observer"
	"github.com/fab-viewer/common-lib/common/bizerror"
)

type engineConfig struct {
	skipPaths []string
	messageFn func(*gin.Context, *bizerror.BizError) string
}

// EngineOption 配置 NewEngine（业务级差异的逃生舱口）。
type EngineOption func(*engineConfig)

// WithSkipPaths 追加访问日志跳过的路径（默认已含 /healthz、/livez、/readyz）。
func WithSkipPaths(paths ...string) EngineOption {
	return func(cfg *engineConfig) {
		cfg.skipPaths = append(cfg.skipPaths, paths...)
	}
}

// WithMessageFunc 改写错误响应 errMsg（透传 errorhandler.WithMessageFunc，
// 如统一脱敏）。
func WithMessageFunc(fn func(*gin.Context, *bizerror.BizError) string) EngineOption {
	return func(cfg *engineConfig) {
		cfg.messageFn = fn
	}
}

// NewEngine 组装引擎：按固定顺序挂四中间件（observer → errorhandler →
// guard → enrich，顺序契约见 api/middleware 包文档）并内置
// /healthz 探活（访问日志默认跳过）。
//
// codeMap 是业务模块错误码 → 响应体业务 code 的映射（每个业务必传，
// 无模块码可传 nil 全按默认 0/1）；其他业务差异经 opts 传入。
func NewEngine(codeMap map[bizerror.Code]int, opts ...EngineOption) *gin.Engine {
	cfg := engineConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	ehOpts := []errorhandler.HandlerOption{errorhandler.WithCodeMap(codeMap)}
	if cfg.messageFn != nil {
		ehOpts = append(ehOpts, errorhandler.WithMessageFunc(cfg.messageFn))
	}

	r := gin.New()
	r.Use(
		observer.Observe(observer.WithSkipPaths(cfg.skipPaths...)),
		errorhandler.ErrorHandler(ehOpts...),
		guard.Guard(),
		enrich.Enrich(),
	)
	r.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return r
}
