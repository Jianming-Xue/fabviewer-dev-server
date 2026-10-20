// Package batch 提供 ClickHouse 批量写入：Batch[T] 按 ch tag 反射追加行，
// 达阈值自动发送（无后台 goroutine，Append 内联触发），Close 兜底发送剩余行。
package batch
