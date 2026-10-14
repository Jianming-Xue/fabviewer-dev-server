// Package middleware 提供 Gin 横切关注点中间件：请求 ID、访问日志、panic 恢复
// 与统一错误处理，仅可被业务系统的接口访问层（及组装根 main）import。
//
// 推荐注册顺序（顺序影响行为，务必保持）：
//
//	r := gin.New()
//	r.Use(
//		middleware.ErrorHandler(log), // 最外层：c.Next() 返回后统一消费 c.Errors
//		middleware.Recovery(log),     // 第二位：兜住后续中间件与路由的 panic
//		middleware.AccessLog(log),    // 访问日志（记录最终状态码与耗时）
//		middleware.RequestID(),       // X-Request-Id 生成/透传/回写
//	)
//
// handler 中约定：发生错误时 c.Error(err) 后直接 return，不要自行写错误响应。
package middleware
