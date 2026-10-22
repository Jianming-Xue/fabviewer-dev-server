// Package middleware 汇总 Gin 横切关注点中间件，按处理流程点分包
// （仅可被业务系统的接口访问层及组装根 main import）：
//
//	observer        访问观察（最外层：在 fault 写回错误响应之后才记录，
//	                status/latency 才是最终值）
//	fault           统一错误出口（c.Next() 返回后统一消费 c.Errors：失败
//	                日志+调用栈，errMsg 直取 err.Error() 写回 Fail）
//	guard           守护兜底（panic → 500 统一体 + 堆栈日志）
//	enrich          上下文丰富（解析五个请求头注入 rctx，预写响应头 txId，
//	                任何位置的写回都能带上）
//
// 推荐注册顺序（顺序影响行为，务必保持）：
//
//	r := gin.New()
//	r.Use(
//		observer.Observe(), // 最外层
//		fault.Fault(),
//		guard.Guard(),
//		enrich.Enrich(),
//	)
//
// handler 中约定：发生错误时 c.Error(err) 后直接 return，不要自行写错误响应。
//
// 本包自身不导出代码，仅承载注册顺序契约与跨四包的全栈集成测试。
package middleware
