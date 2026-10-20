package opensearch

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/common/fverr"
	"github.com/fab-viewer/common-lib/storage"
	"github.com/fab-viewer/common-lib/storage/opensearch/config"
)

// TestConfig_sdkConfig 验证 Config → SDK 配置的映射：地址/凭据透传、
// Pool 默认值与自定义值映射到 http.Transport、Insecure 映射 TLS 配置。
func TestConfig_sdkConfig(t *testing.T) {
	tests := []struct {
		name             string
		cfg              config.Config
		wantAddrs        []string
		wantUser         string
		wantPass         string
		wantInsecure     bool
		wantMaxIdlePerHt int
		wantIdleTimeout  time.Duration
	}{
		{
			name:             "零值 Pool 取默认",
			cfg:              config.Config{Addrs: []string{"http://os1:9200"}},
			wantAddrs:        []string{"http://os1:9200"},
			wantMaxIdlePerHt: 10,
			wantIdleTimeout:  5 * time.Minute,
		},
		{
			name: "自定义 Pool 与凭据",
			cfg: config.Config{
				Addrs:    []string{"https://os1:9200", "https://os2:9200"},
				Username: "admin",
				Password: "secret",
				Pool: storage.PoolConfig{
					MaxIdleConns:    32,
					ConnMaxIdleTime: 30 * time.Second,
				},
			},
			wantAddrs:        []string{"https://os1:9200", "https://os2:9200"},
			wantUser:         "admin",
			wantPass:         "secret",
			wantMaxIdlePerHt: 32,
			wantIdleTimeout:  30 * time.Second,
		},
		{
			name:             "Insecure 跳过 TLS 校验",
			cfg:              config.Config{Addrs: []string{"https://os1:9200"}, Insecure: true},
			wantAddrs:        []string{"https://os1:9200"},
			wantInsecure:     true,
			wantMaxIdlePerHt: 10,
			wantIdleTimeout:  5 * time.Minute,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sdkCfg, err := sdkConfig(tt.cfg)
			require.NoError(t, err)
			assert.Equal(t, tt.wantAddrs, sdkCfg.Addresses)
			assert.Equal(t, tt.wantUser, sdkCfg.Username)
			assert.Equal(t, tt.wantPass, sdkCfg.Password)

			transport, ok := sdkCfg.Transport.(*http.Transport)
			require.True(t, ok, "Transport 应为 *http.Transport")
			assert.Equal(t, tt.wantMaxIdlePerHt, transport.MaxIdleConnsPerHost)
			assert.Equal(t, tt.wantIdleTimeout, transport.IdleConnTimeout)
			if tt.wantInsecure {
				require.NotNil(t, transport.TLSClientConfig)
				assert.True(t, transport.TLSClientConfig.InsecureSkipVerify)
			} else {
				assert.Nil(t, transport.TLSClientConfig, "默认应正常校验 TLS 证书")
			}
		})
	}
}

// TestConfig_sdkConfig_EmptyAddrs 验证空地址列表被拒绝且错误码为 VALIDATION。
func TestConfig_sdkConfig_EmptyAddrs(t *testing.T) {
	_, err := sdkConfig(config.Config{})
	require.Error(t, err)
	require.ErrorIs(t, err, fverr.ErrValidation)
	e, ok := fverr.From(err)
	require.True(t, ok)
	assert.Equal(t, fverr.CodeValidation, e.Code)
}
