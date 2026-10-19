package errs

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	e := New(CodeNotFound, "user not found")
	require.NotNil(t, e)
	assert.Equal(t, CodeNotFound, e.Code)
	assert.Equal(t, "user not found", e.Message)
	assert.Nil(t, e.Err)
	assert.Nil(t, e.Details)
}

func TestNewf(t *testing.T) {
	e := Newf(CodeValidation, "field %s invalid: %d", "email", 3)
	assert.Equal(t, CodeValidation, e.Code)
	assert.Equal(t, "field email invalid: 3", e.Message)
}

func TestWrap(t *testing.T) {
	t.Run("nil 根因返回 nil", func(t *testing.T) {
		assert.Nil(t, Wrap(nil, CodeDB, "query users"))
	})

	t.Run("包装普通错误", func(t *testing.T) {
		root := errors.New("connection refused")
		e := Wrap(root, CodeDB, "query users")
		require.NotNil(t, e)
		assert.Equal(t, CodeDB, e.Code)
		assert.Equal(t, root, e.Unwrap())
	})

	t.Run("显式 code 覆盖底层 Error 的 code", func(t *testing.T) {
		inner := New(CodeInternal, "boom")
		e := Wrap(inner, CodeDB, "call proc")
		assert.Equal(t, CodeDB, e.Code)
		// 根因链仍然可达
		assert.ErrorIs(t, e, inner)
	})
}

func TestWrapf(t *testing.T) {
	assert.Nil(t, Wrapf(nil, CodeDB, "op %s", "x"))
	e := Wrapf(fmt.Errorf("io: %w", errors.New("timeout")), CodeDB, "P_JOB id=%d", 42)
	require.NotNil(t, e)
	assert.Equal(t, "P_JOB id=42", e.Message)
	assert.Equal(t, CodeDB, e.Code)
}

func TestErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		err  *Error
		want string
	}{
		{"无根因", New(CodeNotFound, "user not found"), "[NOT_FOUND] user not found"},
		{"带根因", Wrap(errors.New("io timeout"), CodeDB, "query users"), "[DB_ERROR] query users: io timeout"},
		{"空 code", &Error{Message: "bare"}, "[] bare"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.err.Error())
		})
	}
}

func TestErrorIs(t *testing.T) {
	t.Run("相同 Code 的 sentinel 命中", func(t *testing.T) {
		e := Wrap(errors.New("row gone"), CodeNotFound, "get user")
		assert.ErrorIs(t, e, ErrNotFound)
	})

	t.Run("不同 Code 不命中", func(t *testing.T) {
		e := New(CodeDB, "oops")
		assert.NotErrorIs(t, e, ErrNotFound)
	})

	t.Run("sentinel 直接命中自身", func(t *testing.T) {
		assert.ErrorIs(t, ErrValidation, ErrValidation)
	})

	t.Run("对非 Error 目标返回 false", func(t *testing.T) {
		e := New(CodeInternal, "x")
		assert.False(t, e.Is(errors.New("plain")))
	})

	t.Run("包一层普通错误后仍可经 Unwrap 命中", func(t *testing.T) {
		wrapped := fmt.Errorf("outer: %w", ErrConflict)
		assert.ErrorIs(t, wrapped, ErrConflict)
	})
}

func TestFrom(t *testing.T) {
	t.Run("命中", func(t *testing.T) {
		e, ok := From(fmt.Errorf("ctx: %w", New(CodeTooMany, "slow down")))
		require.True(t, ok)
		assert.Equal(t, CodeTooMany, e.Code)
	})

	t.Run("未命中", func(t *testing.T) {
		_, ok := From(errors.New("plain"))
		assert.False(t, ok)
	})

	t.Run("nil 错误", func(t *testing.T) {
		_, ok := From(nil)
		assert.False(t, ok)
	})
}

func TestWithDetail(t *testing.T) {
	e := New(CodeValidation, "bad request")
	got := e.WithDetail("field", "email").WithDetail("field", "username")
	assert.Equal(t, map[string]any{"field": "username"}, got.Details)
	assert.Same(t, e, got, "链式返回自身")
}

func TestSentinels(t *testing.T) {
	tests := []struct {
		sentinel *Error
		code     Code
	}{
		{ErrInternal, CodeInternal},
		{ErrValidation, CodeValidation},
		{ErrNotFound, CodeNotFound},
		{ErrUnauthorized, CodeUnauthorized},
		{ErrForbidden, CodeForbidden},
		{ErrConflict, CodeConflict},
		{ErrTooMany, CodeTooMany},
		{ErrDB, CodeDB},
		{ErrUpstream, CodeUpstream},
	}
	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			assert.Equal(t, tt.code, tt.sentinel.Code)
			assert.NotEmpty(t, tt.sentinel.Message)
		})
	}
}
