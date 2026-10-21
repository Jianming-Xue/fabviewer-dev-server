// Package runtimecontext 提供请求运行时上下文中间件：解析请求头
// txId/txTime/txName/userName 与标准头 Accept-Language 组装为
// rctx.RuntimeContext 两路注入（请求 context + gin.Context）。
// 应注册在最内层（见 api/middleware 包文档的顺序契约）。
package runtimecontext
