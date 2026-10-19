package clickhouse

import (
	"crypto/tls"
	"testing"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/common/errs"
	"github.com/fab-viewer/common-lib/storage"
	"github.com/fab-viewer/common-lib/storage/clickhouse/config"
)

func TestConfigToOptions(t *testing.T) {
	t.Run("零值字段取默认", func(t *testing.T) {
		opts, err := toOptions(config.Config{Addrs: []string{"ch1:9000", "ch2:9000"}})
		require.NoError(t, err)

		assert.Equal(t, []string{"ch1:9000", "ch2:9000"}, opts.Addr)
		assert.Equal(t, ch.Auth{}, opts.Auth)
		assert.Nil(t, opts.TLS)
		require.NotNil(t, opts.Compression)
		assert.Equal(t, ch.CompressionLZ4, opts.Compression.Method)
		assert.Equal(t, config.DefaultDialTimeout, opts.DialTimeout)

		// 连接池：零值经 WithDefaults 映射
		pool := storage.PoolConfig{}.WithDefaults()
		assert.Equal(t, pool.MaxOpenConns, opts.MaxOpenConns)
		assert.Equal(t, pool.MaxIdleConns, opts.MaxIdleConns)
		assert.Equal(t, pool.ConnMaxLifetime, opts.ConnMaxLifetime)
	})

	t.Run("显式值原样映射", func(t *testing.T) {
		tlsCfg := &tls.Config{ServerName: "ch.internal"}
		cfg := config.Config{
			Addrs:       []string{"ch1:9000"},
			DB:          "analytics",
			User:        "writer",
			Password:    "secret",
			DialTimeout: 3 * time.Second,
			Compression: "zstd",
			TLS:         tlsCfg,
			Pool: storage.PoolConfig{
				MaxOpenConns:    7,
				MaxIdleConns:    3,
				ConnMaxLifetime: 30 * time.Minute,
			},
		}
		opts, err := toOptions(cfg)
		require.NoError(t, err)

		assert.Equal(t, cfg.Addrs, opts.Addr)
		assert.Equal(t, ch.Auth{Database: "analytics", Username: "writer", Password: "secret"}, opts.Auth)
		assert.Same(t, tlsCfg, opts.TLS)
		require.NotNil(t, opts.Compression)
		assert.Equal(t, ch.CompressionZSTD, opts.Compression.Method)
		assert.Equal(t, 3*time.Second, opts.DialTimeout)
		assert.Equal(t, 7, opts.MaxOpenConns)
		assert.Equal(t, 3, opts.MaxIdleConns)
		assert.Equal(t, 30*time.Minute, opts.ConnMaxLifetime)
	})

	t.Run("DialTimeout 非正值取默认", func(t *testing.T) {
		for _, dt := range []time.Duration{0, -time.Second} {
			opts, err := toOptions(config.Config{DialTimeout: dt})
			require.NoError(t, err)
			assert.Equal(t, config.DefaultDialTimeout, opts.DialTimeout)
		}
	})
}

func TestConfigToOptionsCompression(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want ch.CompressionMethod
	}{
		{"空串默认 lz4", "", ch.CompressionLZ4},
		{"小写 lz4", "lz4", ch.CompressionLZ4},
		{"大小写不敏感", "ZSTD", ch.CompressionZSTD},
		{"none 关闭压缩", "none", ch.CompressionNone},
		{"lz4hc", "lz4hc", ch.CompressionLZ4HC},
		{"gzip", "GZip", ch.CompressionGZIP},
		{"deflate", "deflate", ch.CompressionDeflate},
		{"br", "br", ch.CompressionBrotli},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := toOptions(config.Config{Compression: tt.in})
			require.NoError(t, err)
			require.NotNil(t, opts.Compression)
			assert.Equal(t, tt.want, opts.Compression.Method)
		})
	}

	t.Run("非法值返回 CodeValidation", func(t *testing.T) {
		_, err := toOptions(config.Config{Compression: "bogus"})
		var e *errs.Error
		require.ErrorAs(t, err, &e)
		assert.Equal(t, errs.CodeValidation, e.Code)
		assert.Contains(t, err.Error(), `clickhouse.Config compression="bogus" 非法`)
	})
}
