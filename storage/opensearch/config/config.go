package config

import (
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
