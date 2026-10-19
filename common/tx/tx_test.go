package tx

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestContextRoundTrip(t *testing.T) {
	want := Tx{ID: "t-1", Time: time.Date(2026, 10, 19, 17, 30, 0, 0, time.Local), Name: "queryUsers", User: "zhangsan"}
	ctx := NewContext(context.Background(), want)
	assert.Equal(t, want, FromContext(ctx))
}

func TestFromContextZero(t *testing.T) {
	assert.Equal(t, Tx{}, FromContext(context.Background()), "未注入返回零值")
	var nilCtx context.Context
	assert.Equal(t, Tx{}, FromContext(nilCtx), "nil 上下文不 panic")
}

func TestContextOverride(t *testing.T) {
	first := Tx{ID: "t-1", User: "a"}
	second := Tx{ID: "t-2", User: "b"}
	ctx := NewContext(NewContext(context.Background(), first), second)
	assert.Equal(t, second, FromContext(ctx), "后注入覆盖先注入")
}

func TestContextIsolation(t *testing.T) {
	// context.WithValue 不应影响父上下文
	parent := Tx{ID: "parent"}
	child := Tx{ID: "child"}
	pctx := NewContext(context.Background(), parent)
	cctx := NewContext(pctx, child)
	assert.Equal(t, child, FromContext(cctx))
	assert.Equal(t, parent, FromContext(pctx))
}
