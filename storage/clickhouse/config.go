package clickhouse

import (
	"crypto/tls"
	"strings"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"

	"github.com/fab-viewer/common-lib/errs"
	"github.com/fab-viewer/common-lib/storage"
)

// 连接装配的默认值，业务方可直接引用或覆盖。
const (
	// DefaultDialTimeout 是建立连接的默认超时（Config.DialTimeout <=0 时生效）。
	DefaultDialTimeout = 5 * time.Second
	// DefaultCompression 是默认压缩算法名（Config.Compression 为空时生效）。
	DefaultCompression = "lz4"
)

// compressionMethods 是压缩算法名（小写）到 SDK CompressionMethod 的映射。
var compressionMethods = map[string]ch.CompressionMethod{
	"none":    ch.CompressionNone,
	"lz4":     ch.CompressionLZ4,
	"lz4hc":   ch.CompressionLZ4HC,
	"zstd":    ch.CompressionZSTD,
	"gzip":    ch.CompressionGZIP,
	"deflate": ch.CompressionDeflate,
	"br":      ch.CompressionBrotli,
}

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

// toOptions 把 Config 应用默认值后映射为 clickhouse-go/v2 的 Options。
// 压缩名非法时返回 errs.CodeValidation 错误，其余字段不产生错误。
func (c Config) toOptions() (ch.Options, error) { //nolint:gocritic //hugeParam：值语义只读映射，与 Open 的按值 Config 一致
	method := ch.CompressionLZ4
	if c.Compression != "" {
		m, ok := compressionMethods[strings.ToLower(c.Compression)]
		if !ok {
			return ch.Options{}, errs.Newf(errs.CodeValidation,
				"clickhouse.Config compression=%q 非法（可选 none/lz4/lz4hc/zstd/gzip/deflate/br）",
				c.Compression)
		}
		method = m
	}
	dialTimeout := c.DialTimeout
	if dialTimeout <= 0 {
		dialTimeout = DefaultDialTimeout
	}
	pool := c.Pool.WithDefaults()
	return ch.Options{
		Addr:            c.Addrs,
		Auth:            ch.Auth{Database: c.DB, Username: c.User, Password: c.Password},
		TLS:             c.TLS,
		Compression:     &ch.Compression{Method: method},
		DialTimeout:     dialTimeout,
		MaxOpenConns:    pool.MaxOpenConns,
		MaxIdleConns:    pool.MaxIdleConns,
		ConnMaxLifetime: pool.ConnMaxLifetime,
	}, nil
}
