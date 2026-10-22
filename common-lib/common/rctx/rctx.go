package rctx

import (
	"context"
	"time"
)

type ctxKey struct{}

// RuntimeContext 是请求运行时上下文的载荷：接口层从请求头（txId/txTime/
// txName/userName 及标准头 Accept-Language）解析组装，经
// middleware/enrich.Enrich 注入，整个请求期间全链路可读。
//
// 字段全部私有，经 WithXxx 链式赋值（值语义，返回新副本）、FromXxx 取值；
// 定时任务无请求头，入口自注入：
//
//	ctx = rctx.SetRuntimeContext(ctx,
//	    rctx.RuntimeContext{}.WithID(id).WithName("daily-settle"))
//
// User 同时是审计操作者（原 actortx 通道已并入，oracle BasePO 的
// CreatedBy/UpdatedBy 钩子从本字段取值）。
type RuntimeContext struct {
	id   string    // txId
	time time.Time // txTime（yyyyMMDDHHmmss 解析；缺失/解析失败由中间件兜底服务器时间）
	name string    // txName
	user string    // userName（审计操作者）
	lang string    // 请求语言（Accept-Language 首选，规范化如 "zh-CN"；缺失为空串，i18n 按默认语言回退）
}

// WithID 返回设置了 txId 的副本。
func (rc RuntimeContext) WithID(id string) RuntimeContext { //nolint:gocritic //hugeParam：值语义副本链式赋值，同 Config.WithDefaults 模式
	rc.id = id
	return rc
}

// WithTime 返回设置了 txTime 的副本。
func (rc RuntimeContext) WithTime(t time.Time) RuntimeContext { //nolint:gocritic //hugeParam：值语义副本链式赋值
	rc.time = t
	return rc
}

// WithName 返回设置了 txName 的副本。
func (rc RuntimeContext) WithName(name string) RuntimeContext { //nolint:gocritic //hugeParam：值语义副本链式赋值
	rc.name = name
	return rc
}

// WithUser 返回设置了 userName 的副本。
func (rc RuntimeContext) WithUser(user string) RuntimeContext { //nolint:gocritic //hugeParam：值语义副本链式赋值
	rc.user = user
	return rc
}

// WithLang 返回设置了请求语言的副本。
func (rc RuntimeContext) WithLang(lang string) RuntimeContext { //nolint:gocritic //hugeParam：值语义副本链式赋值
	rc.lang = lang
	return rc
}

// FromID 取 txId。
func (rc RuntimeContext) FromID() string { return rc.id } //nolint:gocritic //hugeParam：不可变值快照，指针会开放可变性（同 WithXxx）

// FromTime 取 txTime。
func (rc RuntimeContext) FromTime() time.Time { return rc.time } //nolint:gocritic //hugeParam：不可变值快照

// FromName 取 txName。
func (rc RuntimeContext) FromName() string { return rc.name } //nolint:gocritic //hugeParam：不可变值快照

// FromUser 取 userName（审计操作者）。
func (rc RuntimeContext) FromUser() string { return rc.user } //nolint:gocritic //hugeParam：不可变值快照

// FromLang 取请求语言。
func (rc RuntimeContext) FromLang() string { return rc.lang } //nolint:gocritic //hugeParam：不可变值快照

// SetRuntimeContext 返回携带 RuntimeContext 的上下文（值语义：基于 ctx
// 包一层返回新 context，需接收返回值继续传递）。
//
//nolint:gocritic //hugeParam：RuntimeContext 是请求运行时上下文的不可变值快照，按值传入防止调用方注入后修改
func SetRuntimeContext(ctx context.Context, rc RuntimeContext) context.Context {
	return context.WithValue(ctx, ctxKey{}, rc)
}

// GetRuntimeContext 取出 RuntimeContext；未注入时返回零值。
func GetRuntimeContext(ctx context.Context) RuntimeContext {
	if ctx == nil {
		return RuntimeContext{}
	}
	rc, _ := ctx.Value(ctxKey{}).(RuntimeContext)
	return rc
}
