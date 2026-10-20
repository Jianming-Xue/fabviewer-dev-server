package rctx

import (
	"context"
	"time"
)

type ctxKey struct{}

// RuntimeContext 是请求运行时上下文的载荷：接口层从请求头（txId/txTime/
// txName/userName 及标准头 Accept-Language）解析组装，经
// middleware.FillRuntimeContext 注入，整个请求期间全链路可读。
type RuntimeContext struct {
	ID   string    // txId
	Time time.Time // txTime（yyyyMMDDHHmmss 解析；缺失/解析失败由中间件兜底服务器时间）
	Name string    // txName
	User string    // userName
	Lang string    // 请求语言（Accept-Language 首选，规范化如 "zh-CN"；缺失为空串，i18n 按默认语言回退）
}

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
