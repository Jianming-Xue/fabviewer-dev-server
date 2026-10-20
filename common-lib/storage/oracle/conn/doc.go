// Package conn 提供 Oracle 连接与数据库访问：Open 建池（GORM 装配 + 探活）、
// DB 的三通道出口（GORM ORM / 原生 SQL / 存储过程与函数调用，含事务变体
// CallProcTx/CallFuncTx）、游标 Cursor 与 OUT 绑定标记 Out/OutCursor。
// 驱动差异收敛在内部 driverAdapter 接口之后（默认 godror，CGO + Instant Client）。
package conn
