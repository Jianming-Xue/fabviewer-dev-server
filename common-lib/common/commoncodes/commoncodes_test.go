package commoncodes_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/common/bizerror"
	"github.com/fab-viewer/common-lib/common/commoncodes"
	"github.com/fab-viewer/common-lib/common/i18n"
	"github.com/fab-viewer/common-lib/common/rctx"
)

func TestDictionary(t *testing.T) {
	t.Run("默认英文", func(t *testing.T) {
		e := commoncodes.NewError(context.Background(), commoncodes.ParameterRequired).
			InsertArgs("username")
		_, ok := bizerror.From(e)
		require.True(t, ok)
		assert.Equal(t, "parameter username is required", e.Message)
		assert.Equal(t, commoncodes.ParameterRequired, e.Code)
	})

	t.Run("构造 args 与链式 InsertArgs 等价", func(t *testing.T) {
		e := commoncodes.NewError(context.Background(), commoncodes.ParameterRequired, "username")
		assert.Equal(t, "parameter username is required", e.Message)
	})

	t.Run("请求语言为中文", func(t *testing.T) {
		ctx := rctx.SetRuntimeContext(context.Background(), rctx.RuntimeContext{}.WithLang("zh"))
		e := commoncodes.NewError(ctx, commoncodes.InvalidTime).InsertArgs("from", "bad")
		assert.Contains(t, e.Message, "参数 from")
		assert.Contains(t, e.Message, "bad")
	})

	t.Run("未知语言回退默认英文", func(t *testing.T) {
		ctx := rctx.SetRuntimeContext(context.Background(), rctx.RuntimeContext{}.WithLang("fr"))
		e := commoncodes.NewError(ctx, commoncodes.InternalError)
		assert.Equal(t, "internal error", e.Message)
	})

	t.Run("NewMessage 纯文案出口", func(t *testing.T) {
		ctx := rctx.SetRuntimeContext(context.Background(), rctx.RuntimeContext{}.WithLang("zh"))
		assert.Equal(t, "参数 from 必填",
			commoncodes.NewMessage(ctx, commoncodes.ParameterRequired, "from"))
		// 不带参取模板，调用点经 i18n.InsertArgs 自行替换
		assert.Equal(t, "参数 from 必填",
			i18n.InsertArgs(commoncodes.NewMessage(ctx, commoncodes.ParameterRequired), "from"))
	})
}
