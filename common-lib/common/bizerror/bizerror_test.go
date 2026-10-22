package bizerror

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewError(t *testing.T) {
	t.Run("纯直构", func(t *testing.T) {
		e := NewError(Code("NotFound"), "user not found")
		require.NotNil(t, e)
		assert.Equal(t, Code("NotFound"), e.Code)
		assert.Equal(t, "user not found", e.Message)
		assert.Nil(t, e.Err)
		assert.Nil(t, e.Details)
	})

	t.Run("构造时按序插参", func(t *testing.T) {
		e := NewError(Code("DBError"), "query orders id={0} op={1}", 42, "get")
		assert.Equal(t, "query orders id=42 op=get", e.Message)
	})

	t.Run("无占位符忽略参数", func(t *testing.T) {
		e := NewError(Code("DBError"), "boom", 42)
		assert.Equal(t, "boom", e.Message)
	})

	t.Run("链式插参与构造插参等价", func(t *testing.T) {
		e1 := NewError(Code("DBError"), "id={0}", 7)
		e2 := NewError(Code("DBError"), "id={0}").InsertArgs(7)
		assert.Equal(t, e1.Message, e2.Message)
	})
}

func TestInsertArgs(t *testing.T) {
	tests := []struct {
		name     string
		template string
		args     []any
		want     string
	}{
		{"按序替换", "user {0} not found", []any{42}, "user 42 not found"},
		{"占位符位置随语言不同", "{0} 号用户不存在", []any{42}, "42 号用户不存在"},
		{"多个占位符", "{1} 的用户 {0} 不存在", []any{42, "订单"}, "订单 的用户 42 不存在"},
		{"无占位符忽略参数", "user not found", []any{42}, "user not found"},
		{"参数多于占位符忽略", "user {0}", []any{42, 43}, "user 42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, insertArgs(tt.template, tt.args))
		})
	}

	t.Run("链式返回自身", func(t *testing.T) {
		e := NewError(Code("DBError"), "id={0}")
		got := e.InsertArgs(42)
		assert.Same(t, e, got)
		assert.Equal(t, "id=42", e.Message)
	})
}

func TestAttachError(t *testing.T) {
	root := errors.New("connection refused")
	details := map[string]any{"id": 42}

	e := NewError(Code("DBError"), "query").AttachError(root, details)
	assert.Equal(t, root, e.Err)
	assert.Equal(t, details, e.Details)

	t.Run("nil 明细不覆盖", func(t *testing.T) {
		e := NewError(Code("DBError"), "m")
		e.Details = map[string]any{"k": "v"}
		e.AttachError(root, nil)
		assert.Equal(t, map[string]any{"k": "v"}, e.Details)
	})
}

func TestIsErrorAndFrom(t *testing.T) {
	t.Run("IsError 命中链上任一层", func(t *testing.T) {
		wrapped := fmt.Errorf("outer: %w", NewError(Code("RateLimited"), "slow down"))
		assert.True(t, IsError(wrapped))
		assert.False(t, IsError(errors.New("plain")))
		assert.False(t, IsError(nil))
	})

	t.Run("From 取出对象", func(t *testing.T) {
		e, ok := From(fmt.Errorf("ctx: %w", NewError(Code("RateLimited"), "slow down")))
		require.True(t, ok)
		assert.Equal(t, Code("RateLimited"), e.Code)
		_, ok = From(errors.New("plain"))
		assert.False(t, ok)
		_, ok = From(nil)
		assert.False(t, ok)
	})
}

func TestErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		err  *BizError
		want string
	}{
		{"无根因", NewError(Code("NotFound"), "user not found"), "[NotFound] user not found"},
		{"带根因", NewError(Code("DBError"), "query users").AttachError(errors.New("io timeout"), nil), "[DBError] query users: io timeout"},
		{"空 code", &BizError{Message: "bare"}, "[] bare"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.err.Error())
		})
	}
}

// Is 机制测试用的局部 sentinel：sentinel 本体已移 commoncodes（码的定义
// 不在 bizerror），BizError.Is 只按 Code 比较任意同码实例等价。
var (
	errNotFound = NewError(Code("NotFound"), "not found")
	errDB       = NewError(Code("DBError"), "db error")
	errInvalid  = NewError(Code("InvalidParameter"), "invalid parameter")
)

func TestErrorIs(t *testing.T) {
	t.Run("相同 Code 的 sentinel 命中", func(t *testing.T) {
		e := NewError(Code("NotFound"), "get user").AttachError(errors.New("row gone"), nil)
		assert.ErrorIs(t, e, errNotFound)
	})

	t.Run("链式 BizError 根因同码命中（Is+Unwrap 组合）", func(t *testing.T) {
		// 外层 Code("DBError")、根因 errNotFound：外层 Is() 比码不中 → Unwrap 拆到根因
		// → 根因 Is() 比码命中。删掉 Unwrap 此判断即失效。
		e := NewError(Code("DBError"), "repository.Create").AttachError(errNotFound, nil)
		assert.ErrorIs(t, e, errNotFound)
		assert.ErrorIs(t, e, errDB, "外层自身码同时可判断（链上任一码都命中）")
	})

	t.Run("不同 Code 不命中", func(t *testing.T) {
		assert.NotErrorIs(t, NewError(Code("DBError"), "oops"), errNotFound)
	})

	t.Run("sentinel 直接命中自身", func(t *testing.T) {
		assert.ErrorIs(t, errInvalid, errInvalid)
	})

	t.Run("对非 BizError 目标返回 false", func(t *testing.T) {
		assert.False(t, NewError(Code("InternalError"), "x").Is(errors.New("plain")))
	})

	t.Run("包一层普通错误后仍可经 Unwrap 命中", func(t *testing.T) {
		wrapped := fmt.Errorf("outer: %w", errNotFound)
		assert.ErrorIs(t, wrapped, errNotFound)
	})
}
