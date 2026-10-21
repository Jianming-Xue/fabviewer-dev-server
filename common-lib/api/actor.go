package api

import (
	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/common-lib/common/rctx"
)

// WithActor 把操作者标识（登录用户/任务标识）写入当前请求的 context，
// 供存储层审计字段（oracle.Repository 等）取出。通常在业务认证中间件
// 解析出用户后调用一次：
//
//	func Auth() gin.HandlerFunc {
//	    return func(c *gin.Context) {
//	        userID := parseToken(c) // 业务自行解析
//	        api.WithActor(c, userID)
//	        c.Next()
//	    }
//	}
//
// 定时任务没有 gin 上下文，用 rctx.SetRuntimeContext 直接注入。
func WithActor(c *gin.Context, id string) {
	c.Request = c.Request.WithContext(rctx.SetRuntimeContext(c.Request.Context(),
		rctx.GetRuntimeContext(c.Request.Context()).WithUser(id)))
}
