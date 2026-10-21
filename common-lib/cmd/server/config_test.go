package server_test

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	server "github.com/fab-viewer/common-lib/cmd/server"
)

func TestInit(t *testing.T) {
	t.Cleanup(func() { gin.SetMode(gin.TestMode) })

	t.Run("段缺失全默认并设置 release 模式", func(t *testing.T) {
		cfg, err := server.Init(nil)
		require.NoError(t, err)
		assert.Equal(t, ":8080", cfg.Addr)
		assert.Equal(t, gin.ReleaseMode, cfg.Mode)
		assert.Equal(t, 5*time.Second, cfg.ReadHeaderTimeout)
		assert.Equal(t, 30*time.Second, cfg.ReadTimeout)
		assert.Equal(t, 30*time.Second, cfg.WriteTimeout)
		assert.Equal(t, 120*time.Second, cfg.IdleTimeout)
		assert.Equal(t, 10*time.Second, cfg.ShutdownTimeout)
		assert.Equal(t, gin.ReleaseMode, gin.Mode(), "Init 副作用：设置 gin 模式")
	})

	t.Run("段内键覆盖默认、时长写字符串", func(t *testing.T) {
		cfg, err := server.Init(map[string]any{"server": map[string]any{
			"addr":              ":9090",
			"mode":              "debug",
			"readHeaderTimeout": "5s",
			"shutdownTimeout":   "1s",
			"idleTimeout":       "0s",
		}})
		require.NoError(t, err)
		assert.Equal(t, ":9090", cfg.Addr)
		assert.Equal(t, "debug", cfg.Mode)
		assert.Equal(t, 5*time.Second, cfg.ReadHeaderTimeout)
		assert.Equal(t, 30*time.Second, cfg.ReadTimeout, "未出现的键保留默认")
		assert.Equal(t, time.Duration(0), cfg.IdleTimeout, "0s 表示禁用")
		assert.Equal(t, 1*time.Second, cfg.ShutdownTimeout)
		assert.Equal(t, gin.DebugMode, gin.Mode())
	})

	t.Run("非法 mode 报错", func(t *testing.T) {
		_, err := server.Init(map[string]any{"server": map[string]any{"mode": "prod"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "server.mode")
	})

	t.Run("addr 覆盖为空串报错", func(t *testing.T) {
		_, err := server.Init(map[string]any{"server": map[string]any{"addr": ""}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "server.addr")
	})

	t.Run("负时长报错", func(t *testing.T) {
		_, err := server.Init(map[string]any{"server": map[string]any{"readTimeout": "-1s"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "时长")
	})

	t.Run("非法时长格式报错", func(t *testing.T) {
		_, err := server.Init(map[string]any{"server": map[string]any{"readTimeout": "abc"}})
		require.Error(t, err)
	})
}
