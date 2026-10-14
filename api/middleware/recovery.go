package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/common-lib/api"
	"github.com/fab-viewer/common-lib/errs"
	"github.com/fab-viewer/common-lib/logx"
)

// Recovery 兜住 panic：记录含请求 ID 的错误日志与堆栈，写回 500 统一响应体，
// 防止进程崩溃。响应已写过（panic 前已部分输出）则不再追加写。
// 注册位置应在 ErrorHandler 之后、AccessLog/RequestID 之前（见包文档）。
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
				"request_id", api.RequestIDOf(c),
			)
			if c.Writer.Written() {
				c.Abort()
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, api.Body{
				Code:      errs.CodeInternal,
				Message:   "internal error",
				RequestID: api.RequestIDOf(c),
				Timestamp: time.Now(),
			})
		}()
		c.Next()
	}
}
