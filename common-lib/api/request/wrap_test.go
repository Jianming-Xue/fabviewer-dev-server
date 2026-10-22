package request_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"context"

	"github.com/fab-viewer/common-lib/api/request"
	"github.com/fab-viewer/common-lib/common/bizerror"
	"github.com/fab-viewer/common-lib/common/commoncodes"
	"github.com/fab-viewer/common-lib/common/rctx"
)

// userDTO 模拟 api 层入参对象；userVO 模拟 service 层接收的 VO。
type userDTO struct {
	Name  string
	Level string // 同名异型：字符串数字 → int（WeaklyTypedInput）
	Extra string // VO 没有的字段，应被忽略
}

type userVO struct {
	Name   string
	Level  int
	Unused string // DTO 没有的字段，保持零值
}

type flagDTO struct {
	Name    string
	Enabled int // wire 布尔：1/0
}

type flagVO struct {
	Name    string
	Enabled bool
}

func TestWrap(t *testing.T) {
	t.Run("字段名一致自动拷贝并弱类型转换", func(t *testing.T) {
		vo, err := request.Wrap[userVO](userDTO{Name: "zhangsan", Level: "30"})
		require.NoError(t, err)
		assert.Equal(t, "zhangsan", vo.Name)
		assert.Equal(t, 30, vo.Level, `"30" → 30（WeaklyTypedInput）`)
		assert.Empty(t, vo.Unused, "DTO 缺失的字段保持零值")
	})

	t.Run("布尔按 units.Bool：1 → true、非 1 一律 false", func(t *testing.T) {
		vo, err := request.Wrap[flagVO](flagDTO{Name: "a", Enabled: 1})
		require.NoError(t, err)
		assert.True(t, vo.Enabled, "int 1 → true")

		vo, err = request.Wrap[flagVO](flagDTO{Enabled: 2})
		require.NoError(t, err)
		assert.False(t, vo.Enabled, "int 2 → false，而非「非零即 true」")

		vo, err = request.Wrap[flagVO](flagDTO{})
		require.NoError(t, err)
		assert.False(t, vo.Enabled, "零值 int → false，默认即 false")
	})

	t.Run("无法转换时报错", func(t *testing.T) {
		_, err := request.Wrap[userVO](userDTO{Level: "abc"})
		assert.Error(t, err, "非数字字符串转 int 应报错")
	})
}

func TestWrapCaseInsensitive(t *testing.T) {
	// DTO 字段 Name 与 VO 字段 Username 仅大小写差异（真实场景如
	// user_name → UserName），mapstructure 按忽略大小写匹配。
	type srcDTO struct {
		UserName string
	}
	type dstVO struct {
		Username string
	}
	vo, err := request.Wrap[dstVO](srcDTO{UserName: "lisi"})
	require.NoError(t, err)
	assert.Equal(t, "lisi", vo.Username)
}

func TestRequire(t *testing.T) {
	t.Run("非空返回去空白值", func(t *testing.T) {
		v, err := request.Require(context.Background(), "username", "  zhangsan  ")
		require.NoError(t, err)
		assert.Equal(t, "zhangsan", v)
	})

	t.Run("空串与纯空白报公共错误 ParameterRequired（默认英文）", func(t *testing.T) {
		for _, v := range []string{"", "   "} {
			_, err := request.Require(context.Background(), "username", v)
			require.Error(t, err, "输入 %q 应报错", v)
			e, ok := bizerror.From(err)
			require.True(t, ok)
			assert.Equal(t, commoncodes.ParameterRequired, e.Code)
			assert.Equal(t, "parameter username is required", e.Message)
		}
	})

	t.Run("请求语言为中文时 errMsg 为中文", func(t *testing.T) {
		ctx := rctx.SetRuntimeContext(context.Background(), rctx.RuntimeContext{}.WithLang("zh"))
		_, err := request.Require(ctx, "username", "")
		e, ok := bizerror.From(err)
		require.True(t, ok)
		assert.Equal(t, "参数 username 必填", e.Message)
	})
}
