package reqctx

import (
	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/common-lib/common/rctx"
)

const (
	// HeaderTxID 是事务 ID 的请求/响应头字段名（请求携带则透传，见
	// middleware/runtimecontext.FillRuntimeContext）。
	HeaderTxID = "txId"
	// HeaderTxTime 是事务时间的请求头字段名，格式 yyyyMMDDHHmmss
	// （缺失/非法由中间件兜底服务器时间）。
	HeaderTxTime = "txTime"
	// HeaderTxName 是事务名称的请求头字段名。
	HeaderTxName = "txName"
	// HeaderUserName 是操作用户名的请求头字段名（即审计操作者）。
	HeaderUserName = "userName"
	// HeaderAcceptLanguage 是标准请求语言头（值如 "zh-CN,zh;q=0.9,en;q=0.8"，
	// 由 middleware/runtimecontext.FillRuntimeContext 解析首选语言规范化后
	// 注入 rctx.RuntimeContext 的请求语言）。
	HeaderAcceptLanguage = "Accept-Language"

	// CtxRuntimeContext 是 gin.Context 中存放请求运行时上下文
	// rctx.RuntimeContext 的 key，由 middleware/runtimecontext.FillRuntimeContext
	// 写入。
	CtxRuntimeContext = "commonlib.runtimecontext"
)

// RuntimeContextOf 从 gin.Context 取请求运行时上下文（未启用
// runtimecontext.FillRuntimeContext 中间件时为零值）。
func RuntimeContextOf(c *gin.Context) rctx.RuntimeContext {
	v, ok := c.Get(CtxRuntimeContext)
	if !ok {
		return rctx.RuntimeContext{}
	}
	rc, _ := v.(rctx.RuntimeContext)
	return rc
}

// TxIDOf 从 gin.Context 取事务 ID，是 RuntimeContextOf(c).FromID() 的语法糖
// （未启用 runtimecontext.FillRuntimeContext 中间件时为空字符串）。
func TxIDOf(c *gin.Context) string {
	return RuntimeContextOf(c).FromID()
}

// LangOf 从 gin.Context 取请求语言（规范化如 "zh-CN"；未启用
// runtimecontext.FillRuntimeContext 中间件或请求未携带 Accept-Language
// 时为空字符串），是 RuntimeContextOf(c).FromLang() 的语法糖。
func LangOf(c *gin.Context) string {
	return RuntimeContextOf(c).FromLang()
}

// WithActor 把操作者标识（登录用户/任务标识）写入当前请求的 context，
// 供存储层审计字段（oracle.Repository 等）取出。通常在业务认证中间件
// 解析出用户后调用一次：
//
//	func Auth() gin.HandlerFunc {
//	    return func(c *gin.Context) {
//	        userID := parseToken(c) // 业务自行解析
//	        reqctx.WithActor(c, userID)
//	        c.Next()
//	    }
//	}
//
// 定时任务没有 gin 上下文，用 rctx.SetRuntimeContext 直接注入。
func WithActor(c *gin.Context, id string) {
	c.Request = c.Request.WithContext(rctx.SetRuntimeContext(c.Request.Context(),
		rctx.GetRuntimeContext(c.Request.Context()).WithUser(id)))
}
