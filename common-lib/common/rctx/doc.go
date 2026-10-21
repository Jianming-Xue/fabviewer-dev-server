// Package rctx 提供请求运行时上下文（RuntimeContext）的跨层传递：请求头
// 携带的事务信息（txId/txTime/txName/userName）与标准头 Accept-Language
// 由接口层解析组装为 RuntimeContext，经
// middleware.FillRuntimeContext 注入请求 context，
// service/application/storage 各层经 rctx.GetRuntimeContext 读取。
//
// 与 paging 同理：这是跨层会合点——api 包与 storage 包互不感知，
// 只在本包相会。userName 同时是审计操作者（原独立操作者通道已并入，
// oracle BasePO 钩子直接从 RuntimeContext 取值）。
//
// 字段私有：WithXxx 链式赋值（值语义副本）、FromXxx 取值。定时任务没有
// 请求头，由任务入口自行注入：
//
//	ctx = rctx.SetRuntimeContext(ctx,
//	    rctx.RuntimeContext{}.WithID(id).WithName("daily-settle"))
//
// 本包零三方依赖。
package rctx
