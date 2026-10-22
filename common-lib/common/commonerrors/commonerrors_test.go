package commonerrors_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/common/bizerror"
	"github.com/fab-viewer/common-lib/common/commonerrors"
	"github.com/fab-viewer/common-lib/common/rctx"
)

func TestDictionary(t *testing.T) {
	t.Run("默认英文", func(t *testing.T) {
		e := commonerrors.NewParameterRequired(context.Background()).InsertArgs("username")
		_, ok := bizerror.From(e)
		require.True(t, ok)
		assert.Equal(t, "parameter username is required", e.Message)
		assert.Equal(t, commonerrors.CodeParameterRequired, e.Code)
	})

	t.Run("请求语言为中文", func(t *testing.T) {
		ctx := rctx.SetRuntimeContext(context.Background(), rctx.RuntimeContext{}.WithLang("zh"))
		e := commonerrors.NewInvalidTime(ctx).InsertArgs("from", "bad")
		assert.Contains(t, e.Message, "参数 from")
		assert.Contains(t, e.Message, "bad")
	})

	t.Run("未知语言回退默认英文", func(t *testing.T) {
		ctx := rctx.SetRuntimeContext(context.Background(), rctx.RuntimeContext{}.WithLang("fr"))
		e := commonerrors.NewInternalError(ctx)
		assert.Equal(t, "internal error", e.Message)
	})
}
