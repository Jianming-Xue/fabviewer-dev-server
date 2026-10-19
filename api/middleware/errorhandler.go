package middleware

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fab-viewer/common-lib/api"
	"github.com/fab-viewer/common-lib/common/errs"
	"github.com/fab-viewer/common-lib/common/logx"
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

// WithMessageFunc 改写响应 errMsg（如接 i18n）；返回空串时保留原 errMsg。
func WithMessageFunc(fn func(*errs.Error) string) HandlerOption {
	return func(cfg *handlerConfig) {
		cfg.messageFn = fn
	}
}

// ErrorHandler 统一消费 handler 中经 c.Error(err) 上报的错误并写回统一响应体：
//
//   - *errs.Error：按码表映射状态码（code 即 HTTP 状态码），errMsg 输出消息；
//     字符串错误码与 Details 不出参，随 business error 日志留痕
//   - gorm.ErrRecordNotFound / sql.ErrNoRows → 404
//   - context.Canceled → 499（客户端断开，只记日志）
//   - 其余 → 500，根因只进日志不进响应
//
// 必须最先注册（最外层），c.Next() 返回后才能看到全部错误；
// 响应已写过时跳过（防双写）。log 为 nil 时使用 logx.Default()。
func ErrorHandler(log logx.Logger, opts ...HandlerOption) gin.HandlerFunc {
	cfg := newHandlerConfig(opts)
	if log == nil {
		log = logx.Default()
	}
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 || c.Writer.Written() {
			return
		}
		writeError(c, c.Errors.Last().Err, cfg, log)
	}
}

// newHandlerConfig 以默认码表为底、逐项应用 Option。
func newHandlerConfig(opts []HandlerOption) handlerConfig {
	cfg := handlerConfig{statusMap: make(map[errs.Code]int, len(api.DefaultStatusMap))}
	for code, status := range api.DefaultStatusMap {
		cfg.statusMap[code] = status
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// writeError 将错误翻译为统一响应体写回：优先按 *errs.Error 的码表映射，
// 其余按哨兵错误（not found / canceled）或兜底 500 处理，根因只进日志。
func writeError(c *gin.Context, err error, cfg handlerConfig, log logx.Logger) {
	var e *errs.Error
	if errors.As(err, &e) {
		writeErrsError(c, e, cfg, log)
		return
	}
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound), errors.Is(err, sql.ErrNoRows):
		r := api.NewResponse(c, http.StatusNotFound, "resource not found")
		api.Write(c, http.StatusNotFound, &r)
	case errors.Is(err, context.Canceled):
		log.Info("client canceled request",
			"path", c.Request.URL.Path, "tx_id", api.TxIDOf(c))
		r := api.NewResponse(c, api.StatusClientClosedRequest, "client canceled request")
		api.Write(c, api.StatusClientClosedRequest, &r)
	default:
		log.Error("unhandled error",
			"err", err.Error(),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"tx_id", api.TxIDOf(c))
		r := api.NewResponse(c, http.StatusInternalServerError, "internal error")
		api.Write(c, http.StatusInternalServerError, &r)
	}
}

// writeErrsError 处理 *errs.Error：状态码取码表（未映射按 500，code 即状态码），
// errMsg 可被 messageFn 改写；Details 不出参，随 business error 日志留痕。
func writeErrsError(c *gin.Context, e *errs.Error, cfg handlerConfig, log logx.Logger) {
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
	log.Info("business error",
		"code", string(e.Code),
		"errMsg", message,
		"details", fmt.Sprint(e.Details),
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"tx_id", api.TxIDOf(c))
	r := api.NewResponse(c, status, message)
	api.Write(c, status, &r)
}
