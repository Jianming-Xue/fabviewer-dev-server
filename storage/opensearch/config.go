package opensearch

import (
	"crypto/tls"
	"net/http"

	"github.com/opensearch-project/opensearch-go/v4"

	"github.com/fab-viewer/common-lib/errs"
	"github.com/fab-viewer/common-lib/storage"
)

// Config 是 OpenSearch 连接配置，由业务系统的组装根构造后交给 Open。
// 零值 Pool 经 storage.PoolConfig.WithDefaults 取默认；Addrs 必须至少一项。
type Config struct {
	// Addrs 是节点地址列表（如 https://os1:9200），多项时 SDK 自动轮询。
	Addrs []string
	// Username 是 HTTP Basic Auth 用户名，匿名集群留空。
	Username string
	// Password 是 HTTP Basic Auth 密码，匿名集群留空。
	Password string
	// Insecure 为 true 时跳过 TLS 证书校验，仅限内网自签证书环境显式开启。
	Insecure bool
	// Pool 是 HTTP 连接池配置，映射到 Transport 的空闲连接参数。
	Pool storage.PoolConfig
}

// sdkConfig 将本包 Config 映射为 opensearch-go/v4 的 SDK 配置：
// Pool.MaxIdleConns → MaxIdleConnsPerHost、Pool.ConnMaxIdleTime → IdleConnTimeout
// （均先经 WithDefaults 补默认）。Pool.MaxOpenConns/ConnMaxLifetime 是长连接
// 数据库的池语义，HTTP 连接池按需建连、由 Keep-Alive 复用，无精确对应，不映射。
func (c *Config) sdkConfig() (opensearch.Config, error) {
	if len(c.Addrs) == 0 {
		return opensearch.Config{}, errs.Newf(errs.CodeValidation,
			"opensearch.Config Addrs 不能为空")
	}
	pool := c.Pool.WithDefaults()
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConnsPerHost: pool.MaxIdleConns,
		IdleConnTimeout:     pool.ConnMaxIdleTime,
	}
	if c.Insecure {
		//nolint:gosec // Insecure 由调用方显式 opt-in，仅限内网自签证书集群跳过校验。
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return opensearch.Config{
		Addresses: c.Addrs,
		Username:  c.Username,
		Password:  c.Password,
		Transport: transport,
	}, nil
}
