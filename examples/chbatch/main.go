// Command chbatch 演示 common-lib ClickHouse 封装的完整用法：
// Open（启动期 Ping）→ 建表 → Batch 阈值自动发送 → QuerySlice 读回比对。
//
// 启动（未设置 COMMON_LIB_CH_DSN 时仅打印用法并以 0 退出）：
//
//	export COMMON_LIB_CH_DSN='clickhouse://default:pass@localhost:9000/default'
//	go run ./examples/chbatch
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"

	"github.com/fab-viewer/common-lib/storage/clickhouse"
)

// sample 是示例行结构：ch tag 与表列一一对应。
type sample struct {
	ID   int64     `ch:"id"`
	Name string    `ch:"name"`
	Ts   time.Time `ch:"ts"`
}

const (
	table     = "common_lib_chbatch"
	total     = 1200 // 总写入行数
	threshold = 500  // 自动发送阈值：第 500/1000 行触发，剩余 200 行在 Close 发送
)

func main() {
	dsn := os.Getenv("COMMON_LIB_CH_DSN")
	if dsn == "" {
		fmt.Println("未设置 COMMON_LIB_CH_DSN，仅打印用法：")
		fmt.Println("  export COMMON_LIB_CH_DSN='clickhouse://default:pass@localhost:9000/default'")
		fmt.Println("  go run ./examples/chbatch")
		fmt.Println("流程：Open → 建表 → Batch 写 1200 行（阈值 500 自动发送）→ QuerySlice 读回比对")
		return
	}
	if err := run(dsn); err != nil {
		fmt.Fprintln(os.Stderr, "失败:", err)
		os.Exit(1)
	}
}

func run(dsn string) error {
	opts, err := ch.ParseDSN(dsn)
	if err != nil {
		return fmt.Errorf("解析 DSN 失败: %w", err)
	}
	ctx := context.Background()
	client, err := clickhouse.Open(ctx, clickhouse.Config{
		Addrs:    opts.Addr,
		DB:       opts.Auth.Database,
		User:     opts.Auth.Username,
		Password: opts.Auth.Password,
		TLS:      opts.TLS,
	})
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	if err := prepare(ctx, client); err != nil {
		return err
	}

	if err := write(ctx, client); err != nil {
		return err
	}
	return verify(ctx, client)
}

// prepare 建出幂等可重跑的示例表。
func prepare(ctx context.Context, client *clickhouse.Client) error {
	if err := client.Exec(ctx, `CREATE TABLE IF NOT EXISTS `+table+` (
		id UInt64,
		name String,
		ts DateTime
	) ENGINE = MergeTree ORDER BY id`); err != nil {
		return err
	}
	return client.Exec(ctx, "TRUNCATE TABLE "+table)
}

// write 批量写入 total 行，演示阈值自动发送与 Close 兜底发送。
func write(ctx context.Context, client *clickhouse.Client) error {
	b, err := clickhouse.NewBatch[sample](ctx, client, table, threshold)
	if err != nil {
		return err
	}
	defer func() { _ = b.Close(ctx) }() // 兜底：中途出错也尝试发送已缓冲行

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 1; i <= total; i++ {
		row := sample{
			ID:   int64(i),
			Name: fmt.Sprintf("row-%04d", i),
			Ts:   base.Add(time.Duration(i) * time.Second),
		}
		if err := b.Append(row); err != nil {
			return err
		}
	}
	if err := b.Close(ctx); err != nil { // 剩余行在此发送；defer 的二次 Close 幂等
		return err
	}
	fmt.Printf("写入 %d 行（阈值 %d，自动发送 %d 次，剩余 %d 行在 Close 发送）\n",
		total, threshold, total/threshold, total%threshold)
	return nil
}

// verify 读回全部行并与写入值比对。
func verify(ctx context.Context, client *clickhouse.Client) error {
	got, err := clickhouse.QuerySlice[sample](ctx, client,
		"SELECT id, name, ts FROM "+table+" ORDER BY id")
	if err != nil {
		return err
	}
	if len(got) != total {
		return fmt.Errorf("读回行数不符: want=%d got=%d", total, len(got))
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, row := range got {
		want := sample{
			ID:   int64(i + 1),
			Name: fmt.Sprintf("row-%04d", i+1),
			Ts:   base.Add(time.Duration(i+1) * time.Second),
		}
		// time.Time 含时区信息，用 Equal 比较时刻而非 ==
		if row.ID != want.ID || row.Name != want.Name || !row.Ts.Equal(want.Ts) {
			return fmt.Errorf("第 %d 行不符: want=%+v got=%+v", i+1, want, row)
		}
	}
	fmt.Printf("读回 %d 行，逐行比对一致\n", len(got))
	return nil
}
