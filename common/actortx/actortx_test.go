package actortx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContextRoundTrip(t *testing.T) {
	ctx := SetActorContext(context.Background(), "u-42")
	assert.Equal(t, "u-42", GetActorContext(ctx))
}

func TestGetActorContextEmpty(t *testing.T) {
	assert.Empty(t, GetActorContext(context.Background()), "未注入返回空串")
	var nilCtx context.Context
	assert.Empty(t, GetActorContext(nilCtx), "nil 上下文不 panic")
}

func TestNewContextSkipsEmptyID(t *testing.T) {
	ctx := SetActorContext(context.Background(), "u-1")
	ctx = SetActorContext(ctx, "") // 空值不得覆盖已有操作者
	assert.Equal(t, "u-1", GetActorContext(ctx))
}

func TestContextIsolation(t *testing.T) {
	// context.WithValue 不应影响父上下文
	parent := SetActorContext(context.Background(), "parent-user")
	child := SetActorContext(parent, "child-user")
	assert.Equal(t, "child-user", GetActorContext(child))
	assert.Equal(t, "parent-user", GetActorContext(parent))
}
