package errorhandler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fab-viewer/common-lib/api/errresp"
	"github.com/fab-viewer/common-lib/api/reqctx"
	"github.com/fab-viewer/common-lib/api/response"
	"github.com/fab-viewer/common-lib/common/bizerror"
	"github.com/fab-viewer/common-lib/common/logx"
)

type handlerConfig struct {
	statusMap map[bizerror.Code]int
	messageFn func(*gin.Context, *bizerror.BizError) string
}

// HandlerOption 配置 ErrorHandler 中间件。
type HandlerOption func(*handlerConfig)

// WithStatusMap 以传入映射增补/覆盖默认错误码 → HTTP 状态码表
// （用于业务模块错误码，如 {usererrors.CodeUserDisabled: http.StatusForbidden}）；
// 未映射的自定义码仍按 500 处理。
func WithStatusMap(m map[bizerror.Code]int) HandlerOption {
	return func(cfg *handlerConfig) {
		for code, status := range m {
			cfg.statusMap[code] = status
		}
	}
}

// WithMessageFunc 改写响应 errMsg（逃生舱口，如统一脱敏）；返回空串时保留
// 原 errMsg。BizError 的 Message 在创建时已按请求语言翻译（bizerror.New 查模块
// 字典 + InsertArgs 插参），出口不做翻译。
func WithMessageFunc(fn func(*gin.Context, *bizerror.BizError) string) HandlerOption {
	return func(cfg *handlerConfig) {
		cfg.messageFn = fn
	}
}

// ErrorHandler 统一消费 handler 中经 c.Error(err) 上报的错误并写回统一响应体：
//
//   - *bizerror.BizError：errMsg 直接输出 Message（创建时已翻译，出口零翻译），
//     按码表映射状态码；Code 与 Details 不出参，随 business error 日志留痕
//   - gorm.ErrRecordNotFound / sql.ErrNoRows → 404
//   - context.Canceled → 499（客户端断开，只记日志）
//   - 其余 → 500，根因只进日志不进响应
//
// 必须注册在 accesslog 内层（c.Next() 返回后才能看到全部错误）；
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
	cfg := handlerConfig{statusMap: make(map[bizerror.Code]int, len(errresp.DefaultStatusMap))}
	for code, status := range errresp.DefaultStatusMap {
		cfg.statusMap[code] = status
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// writeError 将错误写回统一响应体：BizError 按码表映射状态码并直接输出
// Message，其余按哨兵错误（not found / canceled）或兜底 500 处理，
// 根因只进日志。
func writeError(c *gin.Context, err error, cfg handlerConfig, log logx.Logger) {
	var e *bizerror.BizError
	if errors.As(err, &e) {
		writeBizError(c, e, cfg, log)
		return
	}
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound), errors.Is(err, sql.ErrNoRows):
		r := response.NewResponse(c, http.StatusNotFound, "resource not found")
		response.Write(c, http.StatusNotFound, &r)
	case errors.Is(err, context.Canceled):
		log.Info("client canceled request",
			"path", c.Request.URL.Path, "tx_id", reqctx.TxIDOf(c))
		r := response.NewResponse(c, errresp.StatusClientClosedRequest, "client canceled request")
		response.Write(c, errresp.StatusClientClosedRequest, &r)
	default:
		log.Error("unhandled error",
			"err", err.Error(),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"tx_id", reqctx.TxIDOf(c))
		r := response.NewResponse(c, http.StatusInternalServerError, "internal error")
		response.Write(c, http.StatusInternalServerError, &r)
	}
}

// writeBizError 处理 *bizerror.BizError：状态码取码表（未映射按 500，code 即状态码），
// errMsg 直接输出 e.Message（不翻译，可被 messageFn 改写）；Details 不出参，
// 随 business error 日志留痕。
func writeBizError(c *gin.Context, e *bizerror.BizError, cfg handlerConfig, log logx.Logger) {
	status, ok := cfg.statusMap[e.Code]
	if !ok {
		status = http.StatusInternalServerError
	}
	message := e.Message
	if cfg.messageFn != nil {
		if rewritten := cfg.messageFn(c, e); rewritten != "" {
			message = rewritten
		}
	}
	log.Info("business error",
		"code", string(e.Code),
		"errMsg", message,
		"details", fmt.Sprint(e.Details),
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"tx_id", reqctx.TxIDOf(c))
	r := response.NewResponse(c, status, message)
	response.Write(c, status, &r)
}
