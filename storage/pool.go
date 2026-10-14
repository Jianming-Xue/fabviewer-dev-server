package storage

import "time"

// 连接池默认值，各存储子包经 PoolConfig.WithDefaults 统一应用。
const (
	// DefaultMaxOpenConns 是 MaxOpenConns 的默认值。
	DefaultMaxOpenConns = 100
	// DefaultMaxIdleConns 是 MaxIdleConns 的默认值。
	DefaultMaxIdleConns = 10
	// DefaultConnMaxLifetime 是 ConnMaxLifetime 的默认值。
	DefaultConnMaxLifetime = time.Hour
	// DefaultConnMaxIdleTime 是 ConnMaxIdleTime 的默认值。
	DefaultConnMaxIdleTime = 5 * time.Minute
)

// PoolConfig 是各存储子包共享的连接池配置（标准库 database/sql 四参数模型）。
// 零值经 WithDefaults 补齐后即为默认池；业务方按需覆盖单个字段。
type PoolConfig struct {
	// MaxOpenConns 是最大打开连接数（含使用中与空闲）。
	MaxOpenConns int
	// MaxIdleConns 是最大空闲连接数。
	MaxIdleConns int
	// ConnMaxLifetime 是连接最长复用时间，到期后连接被关闭重建。
	ConnMaxLifetime time.Duration
	// ConnMaxIdleTime 是连接最长空闲时间，超时的空闲连接被关闭。
	ConnMaxIdleTime time.Duration
}

// WithDefaults 返回补齐零值/非法值后的副本：MaxOpenConns=100、MaxIdleConns=10、
// ConnMaxLifetime=1h、ConnMaxIdleTime=5m；已设置为合法值（>0）的字段保持不变。
func (p PoolConfig) WithDefaults() PoolConfig {
	if p.MaxOpenConns <= 0 {
		p.MaxOpenConns = DefaultMaxOpenConns
	}
	if p.MaxIdleConns <= 0 {
		p.MaxIdleConns = DefaultMaxIdleConns
	}
	if p.ConnMaxLifetime <= 0 {
		p.ConnMaxLifetime = DefaultConnMaxLifetime
	}
	if p.ConnMaxIdleTime <= 0 {
		p.ConnMaxIdleTime = DefaultConnMaxIdleTime
	}
	return p
}
