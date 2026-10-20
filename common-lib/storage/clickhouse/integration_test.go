//go:build integration

package clickhouse_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/storage/clickhouse"
	"github.com/fab-viewer/common-lib/storage/clickhouse/batch"
	"github.com/fab-viewer/common-lib/storage/clickhouse/config"
)

// itRow 与集成测试表列经 ch tag 对应。
type itRow struct {
	ID   int64     `ch:"id"`
	Name string    `ch:"name"`
	Ts   time.Time `ch:"ts"`
}

// configFromDSN 读取 COMMON_LIB_CH_DSN（形如
// clickhouse://user:pass@host:port/db）并转为 Config；未设置时 skip。
func configFromDSN(t *testing.T) config.Config {
	t.Helper()
	dsn := os.Getenv("COMMON_LIB_CH_DSN")
	if dsn == "" {
		t.Skip("COMMON_LIB_CH_DSN 未设置，跳过 ClickHouse 集成测试")
	}
	opts, err := ch.ParseDSN(dsn)
	require.NoError(t, err, "解析 COMMON_LIB_CH_DSN 失败")
	return config.Config{
		Addrs:    opts.Addr,
		DB:       opts.Auth.Database,
		User:     opts.Auth.Username,
		Password: opts.Auth.Password,
		TLS:      opts.TLS,
	}
}

// TestIntegrationBatchAndQuerySlice 是阶段 4 验收线：
// Open → Ping → Batch 写 N 条 → QuerySlice 读回比对。
func TestIntegrationBatchAndQuerySlice(t *testing.T) {
	ctx := context.Background()
	client, err := clickhouse.Open(ctx, configFromDSN(t))
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	require.NoError(t, client.Ping(ctx))

	const table = "common_lib_ch_it"
	require.NoError(t, client.Exec(ctx, `CREATE TABLE IF NOT EXISTS `+table+` (
		id Int64,
		name String,
		ts DateTime
	) ENGINE = MergeTree ORDER BY id`))
	require.NoError(t, client.Exec(ctx, "TRUNCATE TABLE "+table))

	const total, threshold = 120, 50
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	b, err := batch.NewBatch[itRow](ctx, client, table, threshold)
	require.NoError(t, err)

	want := make([]itRow, 0, total)
	for i := 1; i <= total; i++ {
		row := itRow{ID: int64(i), Name: fmt.Sprintf("row-%03d", i), Ts: base.Add(time.Duration(i) * time.Second)}
		require.NoError(t, b.Append(row))
		want = append(want, row)
	}
	require.NoError(t, b.Close(ctx)) // 剩余 20 行在此发送
	require.NoError(t, b.Close(ctx)) // 幂等

	got, err := clickhouse.QuerySlice[itRow](ctx, client,
		"SELECT id, name, ts FROM "+table+" ORDER BY id")
	require.NoError(t, err)
	require.Len(t, got, total)
	for i := range want {
		assert.Equal(t, want[i].ID, got[i].ID)
		assert.Equal(t, want[i].Name, got[i].Name)
		assert.True(t, want[i].Ts.Equal(got[i].Ts),
			"第 %d 行 ts 不符: want=%v got=%v", i+1, want[i].Ts, got[i].Ts)
	}
}
