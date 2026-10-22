package errorhandler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fab-viewer/common-lib/api/response"
	"github.com/fab-viewer/common-lib/common/bizerror"
	"github.com/fab-viewer/common-lib/common/logx"
	"github.com/fab-viewer/common-lib/common/rctx"
)

type handlerConfig struct {
	codeMap   map[bizerror.Code]int
	messageFn func(*gin.Context, *bizerror.BizError) string
}

// HandlerOption 配置 ErrorHandler 中间件。
type HandlerOption func(*handlerConfig)

// WithCodeMap 增补业务模块错误码 → 响应体业务 code 的映射（HTTP 状态码
// 恒 200，映射值进 ApiResponse.Code；如 {usererrors.CodeUserDisabled: 1003}）。
// 未映射的码按 Fail 默认 code 0 处理。
func WithCodeMap(m map[bizerror.Code]int) HandlerOption {
	return func(cfg *handlerConfig) {
		for code, c := range m {
			cfg.codeMap[code] = c
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
//     业务 code 按 WithCodeMap 映射（未映射 0）；Code 与 Details 不出参，
//     随 business error 日志留痕
//   - gorm.ErrRecordNotFound / sql.ErrNoRows → "resource not found"
//   - context.Canceled → "client canceled request"（只记日志）
//   - 其余 → "internal error"，根因只进日志不进响应
//
// HTTP 状态码恒 200（与传输层解耦），写回经 response.Fail（构建响应只有
// OK/Fail 两个入口）。必须注册在 accesslog 内层（c.Next() 返回后才能看到
// 全部错误）；响应已写过时跳过（防双写）。日志经全局出口 logx 记录
// （测试经 logx.SetLogger 注入）。
func ErrorHandler(opts ...HandlerOption) gin.HandlerFunc {
	cfg := newHandlerConfig(opts)
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 || c.Writer.Written() {
			return
		}
		writeError(c, c.Errors.Last().Err, cfg)
	}
}

// newHandlerConfig 以空码表为底、逐项应用 Option（业务码完全由业务经
// WithCodeMap 提供，库内不设默认业务码）。
func newHandlerConfig(opts []HandlerOption) handlerConfig {
	cfg := handlerConfig{codeMap: make(map[bizerror.Code]int)}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// writeError 将错误经 response.Fail 写回：BizError 按码表映射业务 code
// 并直接输出 Message，其余按哨兵错误（not found / canceled）或兜底处理，
// 根因只进日志。
func writeError(c *gin.Context, err error, cfg handlerConfig) {
	var e *bizerror.BizError
	if errors.As(err, &e) {
		writeBizError(c, e, cfg)
		return
	}
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound), errors.Is(err, sql.ErrNoRows):
		response.Fail(c, "resource not found")
	case errors.Is(err, context.Canceled):
		logx.Info("client canceled request",
			"path", c.Request.URL.Path, "tx_id", rctx.GetRuntimeContext(c.Request.Context()).FromID())
		response.Fail(c, "client canceled request")
	default:
		logx.Error("unhandled error",
			"err", err.Error(),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"tx_id", rctx.GetRuntimeContext(c.Request.Context()).FromID())
		response.Fail(c, "internal error")
	}
}

// writeBizError 处理 *bizerror.BizError：业务 code 取 WithCodeMap 码表
// （未映射按 Fail 默认 0），errMsg 直接输出 e.Message（不翻译，可被
// messageFn 改写）；Details 不出参，随 business error 日志留痕。
func writeBizError(c *gin.Context, e *bizerror.BizError, cfg handlerConfig) {
	code, ok := cfg.codeMap[e.Code]
	if !ok {
		code = 0
	}
	message := e.Message
	if cfg.messageFn != nil {
		if rewritten := cfg.messageFn(c, e); rewritten != "" {
			message = rewritten
		}
	}
	logx.Info("business error",
		"code", string(e.Code),
		"errMsg", message,
		"details", fmt.Sprint(e.Details),
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"tx_id", rctx.GetRuntimeContext(c.Request.Context()).FromID())
	response.Fail(c, message, response.WithCode(code))
}
