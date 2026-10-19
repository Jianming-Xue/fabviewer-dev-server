package actor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContextRoundTrip(t *testing.T) {
	ctx := NewContext(context.Background(), "u-42")
	assert.Equal(t, "u-42", FromContext(ctx))
}

func TestFromContextEmpty(t *testing.T) {
	assert.Empty(t, FromContext(context.Background()), "未注入返回空串")
	assert.Empty(t, FromContext(nil), "nil 上下文不 panic")
}

func TestNewContextSkipsEmptyID(t *testing.T) {
	ctx := NewContext(context.Background(), "u-1")
	ctx = NewContext(ctx, "") // 空值不得覆盖已有操作者
	assert.Equal(t, "u-1", FromContext(ctx))
}

func TestContextIsolation(t *testing.T) {
	// context.WithValue 不应影响父上下文
	parent := NewContext(context.Background(), "parent-user")
	child := NewContext(parent, "child-user")
	assert.Equal(t, "child-user", FromContext(child))
	assert.Equal(t, "parent-user", FromContext(parent))
}
