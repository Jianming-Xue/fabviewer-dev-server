package rctx

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithFromRoundTrip(t *testing.T) {
	ts := time.Date(2026, 10, 19, 17, 30, 0, 0, time.Local)
	rc := RuntimeContext{}.
		WithID("t-1").
		WithTime(ts).
		WithName("queryUsers").
		WithUser("zhangsan").
		WithLang("zh-CN")

	assert.Equal(t, "t-1", rc.FromID())
	assert.Equal(t, ts, rc.FromTime())
	assert.Equal(t, "queryUsers", rc.FromName())
	assert.Equal(t, "zhangsan", rc.FromUser())
	assert.Equal(t, "zh-CN", rc.FromLang())

	ctx := SetRuntimeContext(context.Background(), rc)
	got := GetRuntimeContext(ctx)
	assert.Equal(t, rc, got)
	assert.Equal(t, "zhangsan", got.FromUser(), "取出后 FromXxx 照常可用")
}

func TestWithValueSemantics(t *testing.T) {
	base := RuntimeContext{}.WithID("t-1").WithUser("a")

	derived := base.WithUser("b")
	assert.Equal(t, "a", base.FromUser(), "With 返回副本，原值不可变")
	assert.Equal(t, "b", derived.FromUser())
	assert.Equal(t, "t-1", derived.FromID(), "未覆盖字段随副本保留")

	zero := RuntimeContext{}
	assert.Empty(t, zero.FromID())
	assert.True(t, zero.FromTime().IsZero())
	assert.Empty(t, zero.FromUser())
}

func TestGetRuntimeContextZero(t *testing.T) {
	assert.Equal(t, RuntimeContext{}, GetRuntimeContext(context.Background()), "未注入返回零值")
	var nilCtx context.Context
	assert.Equal(t, RuntimeContext{}, GetRuntimeContext(nilCtx), "nil 上下文不 panic")
}

func TestContextOverrideAndIsolation(t *testing.T) {
	first := RuntimeContext{}.WithID("t-1").WithUser("a")
	second := RuntimeContext{}.WithID("t-2").WithUser("b")
	ctx := SetRuntimeContext(SetRuntimeContext(context.Background(), first), second)
	require.Equal(t, "t-2", GetRuntimeContext(ctx).FromID(), "后注入覆盖先注入")

	// context.WithValue 不应影响父上下文
	parent := RuntimeContext{}.WithID("parent")
	child := RuntimeContext{}.WithID("child")
	pctx := SetRuntimeContext(context.Background(), parent)
	cctx := SetRuntimeContext(pctx, child)
	assert.Equal(t, "child", GetRuntimeContext(cctx).FromID())
	assert.Equal(t, "parent", GetRuntimeContext(pctx).FromID())
}
