// Package clickhouse 提供 ClickHouse 访问的轻量封装（基于 clickhouse-go/v2
// 原生接口），仅可被业务系统的存储层（及组装根 main）import。
//
// 能力一览：
//
//	cfg := chcfg.Config{Addrs: []string{"ch1:9000"}, DB: "analytics", User: "default"}
//	client, err := clickhouse.Open(ctx, cfg) // Open 即 Ping，配置/网络问题启动期暴露
//
//	// 批量写：结构体字段经 ch tag 映射到表列
//	b, err := batch.NewBatch[sample](ctx, client, "samples", 1000)
//	defer b.Close(ctx) // 未达阈值的剩余行在此发送
//	err = b.Append(sample{ID: 1, Name: "n", Ts: time.Now()})
//
//	// 查询：整片按 ch tag 反射填充
//	rows, err := clickhouse.QuerySlice[sample](ctx, client, "SELECT id, name, ts FROM samples")
//
// 便捷方法 Exec/Query/Ping 与 SDK 完整能力逃生舱 Conn() 见 Client。
//
// 约定与边界：
//   - 仅使用原生（TCP）协议；Config.Compression 空值默认 lz4，DialTimeout 默认 5s，
//     连接池参数经 storage.PoolConfig 映射（ConnMaxIdleTime 无原生对应项）
//   - 单个 Batch 非并发安全，多 goroutine 需业务侧串行化或各自 NewBatch
//   - 达阈值的自动发送在 Append 内联判断完成，本库不创建任何后台 goroutine
//   - 反射填充路径（Batch.Append / QuerySlice）在包边界 recover，业务方只会拿到 error
//   - 所有错误均包装为带操作上下文的 *errs.Error（存储类错误码 errs.CodeDB）
package clickhouse
