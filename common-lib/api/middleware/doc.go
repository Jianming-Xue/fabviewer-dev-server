// Package middleware 汇总 Gin 横切关注点中间件，按处理流程点分包
// （仅可被业务系统的接口访问层及组装根 main import）：
//
//	accesslog       访问日志（最外层：在 errorhandler 写回错误响应之后才记录，
//	                status/latency 才是最终值）
//	errorhandler    统一错误处理（c.Next() 返回后统一消费 c.Errors）
//	recovery        panic 兜底（500 统一体 + 堆栈日志）
//	runtimecontext  请求运行时上下文（解析五个请求头注入 rctx，预写响应头
//	                txId，任何位置的写回都能带上）
//
// 推荐注册顺序（顺序影响行为，务必保持）：
//
//	r := gin.New()
//	r.Use(
//		accesslog.AccessLog(log),        // 最外层
//		errorhandler.ErrorHandler(log),
//		recovery.Recovery(log),
//		runtimecontext.FillRuntimeContext(),
//	)
//
// handler 中约定：发生错误时 c.Error(err) 后直接 return，不要自行写错误响应。
//
// 本包自身不导出代码，仅承载注册顺序契约与跨四包的全栈集成测试。
package middleware
