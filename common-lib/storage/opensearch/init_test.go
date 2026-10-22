package opensearch_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/storage/opensearch"
)

func TestInit(t *testing.T) {
	t.Run("无 opensearch 段：未启用，返回空句柄与可调用的空 cleanup", func(t *testing.T) {
		cli, cleanup, err := opensearch.Init(map[string]any{"log": map[string]any{}})
		require.NoError(t, err)
		assert.Nil(t, cli)
		require.NotNil(t, cleanup)
		cleanup()
	})

	t.Run("段存在缺 addrs：建连前校验失败", func(t *testing.T) {
		_, _, err := opensearch.Init(map[string]any{"opensearch": map[string]any{
			"username": "u",
		}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "缺 addrs")
	})

	t.Run("段存在：绑定并装配客户端（Open 不拨号）", func(t *testing.T) {
		cli, cleanup, err := opensearch.Init(map[string]any{"opensearch": map[string]any{
			"addrs":    []any{"https://os1:9200", "https://os2:9200"},
			"insecure": true,
		}})
		require.NoError(t, err)
		require.NotNil(t, cli)
		require.NotNil(t, cleanup)
		cleanup() // 空操作，可安全调用
	})
}
