// Package recovery 提供 panic 兜底中间件：记录含事务 ID 的错误日志与堆栈，
// 写回 500 统一响应体，防止进程崩溃。注册位置应在 errorhandler 之后、
// accesslog 与 runtimecontext 之前（见 api/middleware 包文档的顺序契约）。
package recovery
