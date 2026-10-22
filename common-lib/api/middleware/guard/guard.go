package guard

import (
	"fmt"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/common-lib/api/response"
	"github.com/fab-viewer/common-lib/common/commoncodes"
	"github.com/fab-viewer/common-lib/common/logx"
	"github.com/fab-viewer/common-lib/common/rctx"
)

// Guard 兜住 panic（守护）：记录含事务 ID 的错误日志与堆栈，写回公共错误
// InternalError 统一响应体（commoncodes 按请求语言翻译），防止进程崩溃。
// 响应已写过（panic 前已部分输出）则不再追加写。
// 日志经全局出口 logx 记录（测试经 logx.SetLogger 注入），组件零持有日志对象。
func Guard() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}
			logx.Error("panic recovered",
				"error", fmt.Sprintf("%v", r),
				"stack", string(debug.Stack()),
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
				"tx_id", rctx.GetRuntimeContext(c.Request.Context()).FromID(),
			)
			if c.Writer.Written() {
				c.Abort()
				return
			}
			e := commoncodes.NewError(c.Request.Context(), commoncodes.InternalError)
			response.Fail(c, e.Message, response.WithCode(string(e.Code)))
			c.Abort()
		}()
		c.Next()
	}
}
