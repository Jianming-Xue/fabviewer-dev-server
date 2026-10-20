// Package actortx 提供请求上下文中的操作者（当前用户/任务标识）记录与传递：
// api 层从认证信息注入（api.WithActor，或 middleware.FillRuntimeContext 从
// userName 请求头），沿 service/application 层透传，存储层（如 oracle 的
// BasePO 审计钩子）在写库时取出填充审计字段（CreatedBy/UpdatedBy）。
//
// 与 rctx/paging 同理：这是跨层会合点——api 包与 storage 包互不感知，
// 只在本包相会。
//
// 定时任务没有 api 层，由任务入口自行注入：
//
//	ctx = actortx.SetActorContext(ctx, "job:daily-settle")
//
// 本包零三方依赖。
package actortx
