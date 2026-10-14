package middleware

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fab-viewer/common-lib/api"
	"github.com/fab-viewer/common-lib/errs"
	"github.com/fab-viewer/common-lib/logx"
)

type handlerConfig struct {
	statusMap map[errs.Code]int
	messageFn func(*errs.Error) string
}

// HandlerOption 配置 ErrorHandler 中间件。
type HandlerOption func(*handlerConfig)

// WithStatusMap 以传入映射增补/覆盖默认错误码 → HTTP 状态码表
// （用于业务自定义错误码，如 {CodeUserDisabled: http.StatusForbidden}）；
// 未映射的自定义码仍按 500 处理。
func WithStatusMap(m map[errs.Code]int) HandlerOption {
	return func(cfg *handlerConfig) {
		for code, status := range m {
			cfg.statusMap[code] = status
		}
	}
}

// WithMessageFunc 改写响应 message（如接 i18n）；返回空串时保留原 message。
func WithMessageFunc(fn func(*errs.Error) string) HandlerOption {
	return func(cfg *handlerConfig) {
		cfg.messageFn = fn
	}
}

// ErrorHandler 统一消费 handler 中经 c.Error(err) 上报的错误并写回统一响应体：
//
//   - *errs.Error：按码表映射状态码，Message/Details 原样输出
//   - gorm.ErrRecordNotFound / sql.ErrNoRows → 404 NOT_FOUND
//   - context.Canceled → 499（客户端断开，只记日志）
//   - 其余 → 500 INTERNAL_ERROR，根因只进日志不进响应
//
// 必须最先注册（最外层），c.Next() 返回后才能看到全部错误；
// 响应已写过时跳过（防双写）。log 为 nil 时使用 logx.Default()。
func ErrorHandler(log logx.Logger, opts ...HandlerOption) gin.HandlerFunc {
	cfg := handlerConfig{statusMap: make(map[errs.Code]int, len(api.DefaultStatusMap))}
	for code, status := range api.DefaultStatusMap {
		cfg.statusMap[code] = status
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	if log == nil {
		log = logx.Default()
	}
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 || c.Writer.Written() {
			return
		}
		err := c.Errors.Last().Err

		var e *errs.Error
		if errors.As(err, &e) {
			status, ok := cfg.statusMap[e.Code]
			if !ok {
				status = http.StatusInternalServerError
			}
			message := e.Message
			if cfg.messageFn != nil {
				if rewritten := cfg.messageFn(e); rewritten != "" {
					message = rewritten
				}
			}
			b := api.NewBody(c, e.Code, message)
			b.Details = e.Details
			api.Write(c, status, &b)
			return
		}

		switch {
		case errors.Is(err, gorm.ErrRecordNotFound), errors.Is(err, sql.ErrNoRows):
			b := api.NewBody(c, errs.CodeNotFound, "resource not found")
			api.Write(c, http.StatusNotFound, &b)
		case errors.Is(err, context.Canceled):
			log.Info("client canceled request",
				"path", c.Request.URL.Path, "request_id", api.RequestIDOf(c))
			b := api.NewBody(c, errs.CodeInternal, "client canceled request")
			api.Write(c, api.StatusClientClosedRequest, &b)
		default:
			log.Error("unhandled error",
				"err", err.Error(),
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
				"request_id", api.RequestIDOf(c))
			b := api.NewBody(c, errs.CodeInternal, "internal error")
			api.Write(c, http.StatusInternalServerError, &b)
		}
	}
}
