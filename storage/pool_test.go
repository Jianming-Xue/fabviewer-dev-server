package storage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPoolConfigWithDefaults(t *testing.T) {
	tests := []struct {
		name string
		in   PoolConfig
		want PoolConfig
	}{
		{
			name: "零值全部取默认",
			in:   PoolConfig{},
			want: PoolConfig{
				MaxOpenConns:    100,
				MaxIdleConns:    10,
				ConnMaxLifetime: time.Hour,
				ConnMaxIdleTime: 5 * time.Minute,
			},
		},
		{
			name: "非法值（负数）回退默认",
			in: PoolConfig{
				MaxOpenConns:    -1,
				MaxIdleConns:    -5,
				ConnMaxLifetime: -time.Second,
				ConnMaxIdleTime: -time.Minute,
			},
			want: PoolConfig{
				MaxOpenConns:    100,
				MaxIdleConns:    10,
				ConnMaxLifetime: time.Hour,
				ConnMaxIdleTime: 5 * time.Minute,
			},
		},
		{
			name: "已设置的合法值保持不变",
			in: PoolConfig{
				MaxOpenConns:    20,
				MaxIdleConns:    2,
				ConnMaxLifetime: 30 * time.Second,
				ConnMaxIdleTime: 10 * time.Second,
			},
			want: PoolConfig{
				MaxOpenConns:    20,
				MaxIdleConns:    2,
				ConnMaxLifetime: 30 * time.Second,
				ConnMaxIdleTime: 10 * time.Second,
			},
		},
		{
			name: "部分设置仅补齐其余字段",
			in: PoolConfig{
				MaxIdleConns: 7,
			},
			want: PoolConfig{
				MaxOpenConns:    100,
				MaxIdleConns:    7,
				ConnMaxLifetime: time.Hour,
				ConnMaxIdleTime: 5 * time.Minute,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.in.WithDefaults())
		})
	}
}

func TestPoolConfigWithDefaultsDoesNotMutateOriginal(t *testing.T) {
	in := PoolConfig{}
	_ = in.WithDefaults()
	assert.Equal(t, PoolConfig{}, in, "WithDefaults 不应修改原值")
}

func TestPoolConfigDefaultConstants(t *testing.T) {
	assert.Equal(t, 100, DefaultMaxOpenConns)
	assert.Equal(t, 10, DefaultMaxIdleConns)
	assert.Equal(t, time.Hour, DefaultConnMaxLifetime)
	assert.Equal(t, 5*time.Minute, DefaultConnMaxIdleTime)
}
