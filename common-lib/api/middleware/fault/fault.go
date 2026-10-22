package fault

import (
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/common-lib/api/response"
	"github.com/fab-viewer/common-lib/common/bizerror"
	"github.com/fab-viewer/common-lib/common/logx"
	"github.com/fab-viewer/common-lib/common/rctx"
)

type faultConfig struct {
	codeMap map[bizerror.Code]int
}

// Option 配置 Fault 中间件。
type Option func(*faultConfig)

// WithCodeMap 增补业务模块错误码 → 响应体业务 code 的映射（HTTP 状态码
// 恒 200，映射值进 ApiResponse.Code；如 {usererrors.CodeUserDisabled: 1003}）。
// 未映射的码按 Fail 默认 code 0 处理。
func WithCodeMap(m map[bizerror.Code]int) Option {
	return func(cfg *faultConfig) {
		for code, c := range m {
			cfg.codeMap[code] = c
		}
	}
}

// Fault 统一消费 handler 链经 c.Error(err) 上报的错误（不区分 BizError
// 与否）：记录失败日志与调用栈（BizError 附业务码与 Details 留痕），并经
// response.Fail 写回错误响应——**errMsg 直取 err.Error()，出口不做任何
// 翻译或固定文案替换**；code 按 WithCodeMap 映射（BizError 未映射与其余
// 错误一律 0）。
//
// 必须注册在 observer 内层（c.Next() 返回后才能看到全部错误）、guard 外层；
// 响应已写过时跳过（防双写）。日志经全局出口 logx 记录（测试经
// logx.SetLogger 注入），组件零持有日志对象。
func Fault(opts ...Option) gin.HandlerFunc {
	cfg := newConfig(opts)
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 || c.Writer.Written() {
			return
		}
		err := c.Errors.Last().Err
		logx.Error("request fault",
			"err", err.Error(),
			"bizCode", bizCodeOf(err),
			"details", bizDetailsOf(err),
			"stack", string(debug.Stack()),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"tx_id", rctx.GetRuntimeContext(c.Request.Context()).FromID())
		response.Fail(c, errMsgOf(err), response.WithCode(cfg.codeOf(err)))
	}
}

// newConfig 以空码表为底、逐项应用 Option（业务码完全由业务经
// WithCodeMap 提供，库内不设默认业务码）。
func newConfig(opts []Option) faultConfig {
	cfg := faultConfig{codeMap: make(map[bizerror.Code]int)}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// codeOf 返回错误的响应体业务 code：BizError 查码表（未映射 0），其余 0。
func (cfg faultConfig) codeOf(err error) int {
	var e *bizerror.BizError
	if errors.As(err, &e) {
		if c, ok := cfg.codeMap[e.Code]; ok {
			return c
		}
	}
	return 0
}

// errMsgOf 返回响应 errMsg：BizError 取 Message（构造时已定稿的业务消息；
// err.Error() 的 "[CODE] msg: root" 是内部日志形态，不出参），其余取
// err.Error() 原文。
func errMsgOf(err error) string {
	var e *bizerror.BizError
	if errors.As(err, &e) {
		return e.Message
	}
	return err.Error()
}

// bizCodeOf 返回 BizError 的字符串错误码（非 BizError 为空串），供日志留痕。
func bizCodeOf(err error) string {
	var e *bizerror.BizError
	if errors.As(err, &e) {
		return string(e.Code)
	}
	return ""
}

// bizDetailsOf 返回 BizError 的 Details（非 BizError 为空串），不出响应体。
func bizDetailsOf(err error) string {
	var e *bizerror.BizError
	if errors.As(err, &e) {
		return fmt.Sprint(e.Details)
	}
	return ""
}
