// Package tx 提供请求运行时上下文的跨层传递：请求头携带的事务信息
// （txId/txTime/txName/userName）由接口层解析组装为 Tx，经
// middleware.RuntimeContext 注入请求 context，service/application/storage
// 各层经 tx.FromContext 读取。
//
// 与 actor/paging 同理：这是跨层会合点——api 包与 storage 包互不感知，
// 只在本包相会。userName 的审计用途由 actor 包承接（中间件注入时同链写入），
// 本包只负责事务信息的完整传递。
//
// 定时任务没有请求头，由任务入口自行注入：
//
//	ctx = tx.NewContext(ctx, tx.Tx{ID: snowflake.Next(), Name: "daily-settle"})
//
// 本包零三方依赖。
package tx
