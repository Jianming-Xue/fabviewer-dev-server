package tx

import (
	"context"
	"time"
)

type ctxKey struct{}

// Tx 是请求运行时上下文的载荷：接口层从请求头（txId/txTime/txName/userName）
// 解析组装，经 middleware.RuntimeContext 注入，整个请求期间全链路可读。
type Tx struct {
	ID   string    // txId
	Time time.Time // txTime（yyyyMMDDHHmmss 解析；缺失/解析失败由中间件兜底服务器时间）
	Name string    // txName
	User string    // userName
}

// NewContext 返回携带 Tx 的上下文。
func NewContext(ctx context.Context, t Tx) context.Context {
	return context.WithValue(ctx, ctxKey{}, t)
}

// FromContext 取出 Tx；未注入时返回零值。
func FromContext(ctx context.Context) Tx {
	if ctx == nil {
		return Tx{}
	}
	t, _ := ctx.Value(ctxKey{}).(Tx)
	return t
}
