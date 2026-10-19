package config

import (
	"crypto/tls"
	"time"

	"github.com/fab-viewer/common-lib/storage"
)

// 连接装配的默认值，业务方可直接引用或覆盖。
const (
	// DefaultDialTimeout 是建立连接的默认超时（Config.DialTimeout <=0 时生效）。
	DefaultDialTimeout = 5 * time.Second
	// DefaultCompression 是默认压缩算法名（Config.Compression 为空时生效）。
	DefaultCompression = "lz4"
)

// Config 是 ClickHouse 连接配置。零值字段取默认：DialTimeout 5s、Compression
// lz4、连接池参数经 storage.PoolConfig.WithDefaults 补齐；本封装仅使用原生
// （TCP）协议。
type Config struct {
	// Addrs 是节点地址列表（host:port）；为空时由 SDK 默认 localhost:9000。
	Addrs []string
	// DB 是默认数据库名，可为空。
	DB string
	// User 是用户名，为空时由 SDK 默认 default。
	User string
	// Password 是密码。
	Password string
	// DialTimeout 是单次拨号超时，<=0 时取 DefaultDialTimeout。
	DialTimeout time.Duration
	// Compression 是压缩算法名（none/lz4/lz4hc/zstd/gzip/deflate/br，大小写
	// 不敏感），为空时取 DefaultCompression；非法值在 Open 时返回
	// errs.CodeValidation 错误。
	Compression string
	// TLS 是可选 TLS 配置，nil 表示明文连接。
	TLS *tls.Config
	// Pool 是连接池参数：MaxOpenConns/MaxIdleConns/ConnMaxLifetime 映射到 SDK；
	// ConnMaxIdleTime 在原生协议 SDK 中无对应项（其连接池按生命周期淘汰），
	// 字段被接受但不映射。
	Pool storage.PoolConfig
}
