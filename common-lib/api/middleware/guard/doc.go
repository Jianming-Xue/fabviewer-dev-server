// Package guard 提供守护中间件：panic 兜底：记录含事务 ID 的错误日志与堆栈，
// 写回 500 统一响应体，防止进程崩溃。注册位置应在 fault 之后、
// observer 与 enrich 之前（见 api/middleware 包文档的顺序契约）。
package guard
