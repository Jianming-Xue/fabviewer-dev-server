// Package rctx 提供请求运行时上下文（RuntimeContext）的跨层传递：请求头
// 携带的事务信息（txId/txTime/txName/userName）与标准头 Accept-Language
// 由接口层解析组装为 RuntimeContext，经
// middleware.FillRuntimeContext 注入请求 context，
// service/application/storage 各层经 rctx.GetRuntimeContext 读取。
//
// 与 actortx/paging 同理：这是跨层会合点——api 包与 storage 包互不感知，
// 只在本包相会。userName 的审计用途由 actortx 包承接（中间件注入时同链写入），
// 本包只负责运行时上下文的完整传递。
//
// 定时任务没有请求头，由任务入口自行注入：
//
//	ctx = rctx.SetRuntimeContext(ctx, rctx.RuntimeContext{ID: snowflake.Next(), Name: "daily-settle"})
//
// 本包零三方依赖。
package rctx
