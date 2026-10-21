// Package reqctx 是请求运行时上下文在 gin 侧的绑定：wire 契约（五个请求头
// 字段名）、gin.Context 的存取 key、取值语法糖，以及业务认证中间件写入
// 操作者的 WithActor。
//
// 组装方是 middleware/runtimecontext.FillRuntimeContext（解析请求头注入
// rctx.RuntimeContext）；业务 handler 经 RuntimeContextOf/TxIDOf/LangOf
// 读取（未启用该中间件时为零值），存储层审计经 rctx.GetRuntimeContext
// 从请求 context 取（userName 即审计操作者）。
package reqctx
