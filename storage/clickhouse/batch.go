package clickhouse

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/fab-viewer/common-lib/errs"
)

//go:generate go tool mockgen -source=batch.go -destination=batch_mock_test.go -package=clickhouse -mock_names=prepareBatcher=MockPrepareBatcher,batchSender=MockBatchSender

// prepareBatcher 抽象建立批量 INSERT 所需的最小 SDK 接口（driver.Conn 子集，
// 返回值收窄为 batchSender），供 NewBatch 使用与单测替身实现。
type prepareBatcher interface {
	// PrepareBatch 预建一条 INSERT 语句的批量写通道。
	PrepareBatch(ctx context.Context, query string, opts ...driver.PrepareBatchOption) (batchSender, error)
}

// batchSender 抽象批量写入所需的最小 SDK 接口（driver.Batch 子集）：
// AppendStruct 按 ch tag 反射追加一行，Flush 发送缓冲但保持可用，
// Send 发送并终结 INSERT，Close 释放底层资源。
type batchSender interface {
	AppendStruct(v any) error
	Flush() error
	Send() error
	Close() error
}

// connBatcher 把 driver.Conn 适配为 prepareBatcher（driver.Batch 天然满足
// batchSender）。
type connBatcher struct{ conn driver.Conn }

// PrepareBatch 实现 prepareBatcher。
func (a connBatcher) PrepareBatch(
	ctx context.Context, query string, opts ...driver.PrepareBatchOption,
) (batchSender, error) {
	return a.conn.PrepareBatch(ctx, query, opts...)
}

var _ prepareBatcher = connBatcher{} // 编译期断言：适配器满足接口

// Batch 是泛型批量写入器：缓冲 T 行，达到阈值自动发送，Close 时发送剩余行
// 并终结 INSERT。T 必须为结构体，字段经 ch tag 映射到表列（与
// rows.ScanStruct 同一套映射规则）。
//
// 单个 Batch 非并发安全：多 goroutine 写入需在业务侧串行化；
// 并发场景请每 goroutine 独立 NewBatch。
type Batch[T any] struct {
	sender batchSender
	table  string // 目标表名，用于错误上下文
	size   int    // 自动发送阈值，<=0 表示不自动发送
	count  int    // 自上次发送后累计的缓冲行数
	closed bool   // Close 后进入终态，所有后续操作短路
}

// NewBatch 为 table 建立泛型批量写入通道（底层执行 INSERT INTO table）。
// size 是自动发送阈值：缓冲行数达到 size 时，Append 内部自动发送已缓冲行
// （内联判断，无后台 goroutine）并继续接收后续行；size<=0 表示不自动发送，
// 仅在显式 Flush/Close 时发送。table 直接拼入 INSERT 语句，必须是业务侧
// 可信的表名（不要透传外部输入）。
func NewBatch[T any](ctx context.Context, c *Client, table string, size int) (*Batch[T], error) {
	return newBatch[T](ctx, connBatcher{conn: c.conn}, table, size)
}

// newBatch 是 NewBatch 的可注入版本（单测传入 prepareBatcher 替身）。
func newBatch[T any](ctx context.Context, pb prepareBatcher, table string, size int) (b *Batch[T], err error) {
	defer func() { // 反射/网络路径在包边界 recover，不向业务方泄漏 panic
		if r := recover(); r != nil {
			b, err = nil, errs.Newf(errs.CodeDB, "clickhouse.NewBatch table=%s: %v", table, r)
		}
	}()
	sender, err := pb.PrepareBatch(ctx, "INSERT INTO "+table)
	if err != nil {
		return nil, errs.Wrapf(err, errs.CodeDB, "clickhouse.NewBatch table=%s", table)
	}
	return &Batch[T]{sender: sender, table: table, size: size}, nil
}

// Append 追加一行（经 ch tag 反射填充）。缓冲达到 size 阈值时自动发送已
// 缓冲行并重置计数，Batch 继续可用。追加失败或自动发送失败均返回
// errs.CodeDB 错误；自动发送失败时计数不重置，下次 Append 或显式 Flush 会
// 重试发送。Close 之后调用返回错误。
func (b *Batch[T]) Append(v T) error {
	if b.closed {
		return errs.Newf(errs.CodeDB, "clickhouse.Batch.Append table=%s: batch 已关闭", b.table)
	}
	if err := b.append(v); err != nil {
		return err
	}
	if b.size > 0 && b.count >= b.size {
		if err := b.send(); err != nil {
			return errs.Wrapf(err, errs.CodeDB,
				"clickhouse.Batch.Append table=%s size=%d 达到阈值自动发送失败", b.table, b.size)
		}
	}
	return nil
}

// append 调用底层 AppendStruct 并累计计数。
func (b *Batch[T]) append(v T) (err error) {
	defer func() { // 反射路径在包边界 recover
		if r := recover(); r != nil {
			err = errs.Newf(errs.CodeDB, "clickhouse.Batch.Append table=%s: %v", b.table, r)
		}
	}()
	if err := b.sender.AppendStruct(&v); err != nil {
		return errs.Wrapf(err, errs.CodeDB, "clickhouse.Batch.Append table=%s", b.table)
	}
	b.count++
	return nil
}

// Flush 显式发送当前缓冲行，Batch 保持可用（可继续 Append）。
// ctx 已取消时直接返回错误，不发起发送。Close 之后调用返回错误。
func (b *Batch[T]) Flush(ctx context.Context) error {
	if b.closed {
		return errs.Newf(errs.CodeDB, "clickhouse.Batch.Flush table=%s: batch 已关闭", b.table)
	}
	if err := ctx.Err(); err != nil {
		return errs.Wrapf(err, errs.CodeDB, "clickhouse.Batch.Flush table=%s: ctx 已取消", b.table)
	}
	if err := b.send(); err != nil {
		return errs.Wrapf(err, errs.CodeDB, "clickhouse.Batch.Flush table=%s", b.table)
	}
	return nil
}

// send 调用底层 Flush 发送缓冲行（保持 INSERT 可用）并重置计数。
// 不做错误包装，由调用方附加操作上下文。
func (b *Batch[T]) send() error {
	if err := b.sender.Flush(); err != nil {
		return err
	}
	b.count = 0
	return nil
}

// Close 发送剩余缓冲行（终结 INSERT）并释放底层资源。幂等：重复调用返回
// nil；这是未达阈值缓冲行的最终发送点，建议 defer 调用。
// ctx 已取消时不发送缓冲行，仅释放底层连接并返回错误。
func (b *Batch[T]) Close(ctx context.Context) error {
	if b.closed {
		return nil
	}
	b.closed = true // 先置终态：即使失败也不再重试，保证幂等语义确定
	if err := ctx.Err(); err != nil {
		_ = b.sender.Close() // 释放底层连接（不发送）
		return errs.Wrapf(err, errs.CodeDB, "clickhouse.Batch.Close table=%s: ctx 已取消", b.table)
	}
	sendErr := b.sender.Send()
	closeErr := b.sender.Close() // SDK 语义：Send 后 Close 为安全的收尾
	if sendErr != nil {
		return wrapDB(sendErr, "clickhouse.Batch.Close table=%s Send", b.table)
	}
	return wrapDB(closeErr, "clickhouse.Batch.Close table=%s Close", b.table)
}
