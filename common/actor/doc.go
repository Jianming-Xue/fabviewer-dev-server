// Package actor 提供请求上下文中的操作者（当前用户/任务标识）传递：
// api 层从认证信息注入（api.WithActor），沿 service/application 层透传，
// 存储层（如 oracle.Repository）在写库时取出填充审计字段。
//
// 与 paging 同理：这是跨层会合点——api 包与 storage 包互不感知，只在本包相会。
//
// 定时任务没有 api 层，由任务入口自行注入：
//
//	ctx = actor.NewContext(ctx, "job:daily-settle")
//
// 本包零三方依赖。
package actor
