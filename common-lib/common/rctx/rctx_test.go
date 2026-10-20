package rctx

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestContextRoundTrip(t *testing.T) {
	want := RuntimeContext{ID: "t-1", Time: time.Date(2026, 10, 19, 17, 30, 0, 0, time.Local), Name: "queryUsers", User: "zhangsan"}
	ctx := SetRuntimeContext(context.Background(), want)
	assert.Equal(t, want, GetRuntimeContext(ctx))
}

func TestGetRuntimeContextZero(t *testing.T) {
	assert.Equal(t, RuntimeContext{}, GetRuntimeContext(context.Background()), "未注入返回零值")
	var nilCtx context.Context
	assert.Equal(t, RuntimeContext{}, GetRuntimeContext(nilCtx), "nil 上下文不 panic")
}

func TestContextOverride(t *testing.T) {
	first := RuntimeContext{ID: "t-1", User: "a"}
	second := RuntimeContext{ID: "t-2", User: "b"}
	ctx := SetRuntimeContext(SetRuntimeContext(context.Background(), first), second)
	assert.Equal(t, second, GetRuntimeContext(ctx), "后注入覆盖先注入")
}

func TestContextIsolation(t *testing.T) {
	// context.WithValue 不应影响父上下文
	parent := RuntimeContext{ID: "parent"}
	child := RuntimeContext{ID: "child"}
	pctx := SetRuntimeContext(context.Background(), parent)
	cctx := SetRuntimeContext(pctx, child)
	assert.Equal(t, child, GetRuntimeContext(cctx))
	assert.Equal(t, parent, GetRuntimeContext(pctx))
}
