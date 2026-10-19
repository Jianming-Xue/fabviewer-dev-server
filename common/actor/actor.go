package actor

import "context"

type ctxKey struct{}

// NewContext 返回携带操作者标识的上下文。id 为空时原样返回（避免空值覆盖）。
func NewContext(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, id)
}

// FromContext 取出操作者标识；未注入时返回空字符串。
func FromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(ctxKey{}).(string)
	return v
}
