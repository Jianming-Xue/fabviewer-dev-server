// Package middleware 提供 Gin 横切关注点中间件：请求运行时上下文、访问日志、
// panic 恢复与统一错误处理，仅可被业务系统的接口访问层（及组装根 main）import。
//
// 推荐注册顺序（顺序影响行为，务必保持）：
//
//	r := gin.New()
//	r.Use(
//		middleware.AccessLog(log),        // 最外层：在 ErrorHandler 写回错误响应之后
//		                                  // 才记录，status/latency 才是最终值
//		middleware.ErrorHandler(log),     // c.Next() 返回后统一消费 c.Errors
//		middleware.Recovery(log),         // 兜住后续中间件与路由的 panic
//		middleware.FillRuntimeContext(), // 解析请求头 txId/txTime/txName/
//		                                  // userName 与标准头 Accept-Language
//		                                  // 注入运行时上下文（预写响应头 txId，
//		                                  // 任何位置的写回都能带上）
//	)
//
// handler 中约定：发生错误时 c.Error(err) 后直接 return，不要自行写错误响应。
//
// 注意：AccessLog 必须在 ErrorHandler 外层，否则错误路径的状态码（404/500）
// 会按 gin 默认值 200 记录。
package middleware
