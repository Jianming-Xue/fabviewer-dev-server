package actortx

import "context"

type ctxKey struct{}

// SetActorContext 返回携带操作者标识的上下文。id 为空时原样返回
// （避免空值覆盖）；值语义：基于 ctx 包一层返回新 context，需接收返回值
// 继续传递。
func SetActorContext(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, id)
}

// GetActorContext 取出操作者标识；未注入时返回空字符串。
func GetActorContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(ctxKey{}).(string)
	return v
}
