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

// Fault 统一消费 handler 链经 c.Error(err) 上报的错误（不区分 BizError
// 与否）：记录失败日志与调用栈（BizError 附业务码与 Details 留痕），并经
// response.Fail 写回错误响应——**errMsg 直取错误消息，出口不做任何翻译
// 或固定文案替换**（BizError 取 Message 字段，其余取 err.Error() 原文）；
// **code：BizError 直接透传其 bizerror.Code 字符串**（与 ApiResponse.Code
// 同一命名空间），其余按 Fail 默认 "FAIL"。
//
// 必须注册在 observer 内层（c.Next() 返回后才能看到全部错误）、guard 外层；
// 响应已写过时跳过（防双写）。日志经全局出口 logx 记录（测试经
// logx.SetLogger 注入），组件零持有日志对象。
func Fault() gin.HandlerFunc {
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
		if code := codeOf(err); code != "" {
			response.Fail(c, errMsgOf(err), response.WithCode(code))
			return
		}
		response.Fail(c, errMsgOf(err))
	}
}

// codeOf 返回错误的响应体业务 code：BizError 直接透传其 code 字符串，
// 其余为空串（Fail 兜底为默认 "FAIL"）。
func codeOf(err error) string {
	var e *bizerror.BizError
	if errors.As(err, &e) {
		return string(e.Code)
	}
	return ""
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
