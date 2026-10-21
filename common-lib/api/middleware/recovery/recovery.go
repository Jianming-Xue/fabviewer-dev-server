package recovery

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/common-lib/api/reqctx"
	"github.com/fab-viewer/common-lib/api/response"
	"github.com/fab-viewer/common-lib/common/logx"
)

// Recovery 兜住 panic：记录含事务 ID 的错误日志与堆栈，写回 500 统一响应体，
// 防止进程崩溃。响应已写过（panic 前已部分输出）则不再追加写。
// 日志经全局出口 logx 记录（测试经 logx.SetLogger 注入），组件零持有日志对象。
func Recovery() gin.HandlerFunc {
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
				"tx_id", reqctx.TxIDOf(c),
			)
			if c.Writer.Written() {
				c.Abort()
				return
			}
			resp := response.NewResponse(c, http.StatusInternalServerError, "internal error")
			c.AbortWithStatusJSON(http.StatusInternalServerError, resp)
		}()
		c.Next()
	}
}
