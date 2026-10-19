package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/common-lib/api"
	"github.com/fab-viewer/common-lib/common/logx"
)

// Recovery 兜住 panic：记录含事务 ID 的错误日志与堆栈，写回 500 统一响应体，
// 防止进程崩溃。响应已写过（panic 前已部分输出）则不再追加写。
// 注册位置应在 ErrorHandler 之后、AccessLog/TxID 之前（见包文档）。
func Recovery(log logx.Logger) gin.HandlerFunc {
	if log == nil {
		log = logx.Default()
	}
	return func(c *gin.Context) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}
			log.Error("panic recovered",
				"error", fmt.Sprintf("%v", r),
				"stack", string(debug.Stack()),
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
				"tx_id", api.TxIDOf(c),
			)
			if c.Writer.Written() {
				c.Abort()
				return
			}
			resp := api.NewResponse(c, http.StatusInternalServerError, "internal error")
			c.AbortWithStatusJSON(http.StatusInternalServerError, resp)
		}()
		c.Next()
	}
}
