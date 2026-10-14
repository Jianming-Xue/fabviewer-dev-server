// Package clickhouse 提供 ClickHouse 访问的轻量封装（基于 clickhouse-go/v2 原生
// 接口）：连接装配、Exec/Query、泛型批量写入 Batch[T] 与查询填充 QuerySlice[T]，
// 仅可被业务系统的存储层（及组装根 main）import。
//
// SDK 完整能力经 Conn() 逃生舱暴露；包装层只做连接管理与高频操作助手。
// 注意：单个 Batch 非并发安全，多 goroutine 写入需在业务侧串行化。
package clickhouse
