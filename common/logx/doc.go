// Package logx 提供完整日志工具：zap 引擎 + 可组合出口（标准输出/文件
// lumberjack 轮转/远程批量出口），并保留最小 Logger 接口作为库内组件
// （中间件、存储包装）与业务系统的统一接缝。
//
// 业务入口一行接入（stdout+文件；远程出口可再加）：
//
//	log, closeLog, err := logx.New(logx.Config{
//	    Stdout: true,
//	    File:   &logx.FileConfig{Path: "app.log", MaxSizeMB: 100, MaxBackups: 7},
//	    Remote: logsink, // 可选：storage/opensearch/logsink 直写 OpenSearch
//	})
//	defer closeLog() // 优雅停机处调用：排空远程缓冲、关闭文件
//	logx.SetDefault(log)
//
// 远程出口为异步批量（默认 256 行/批或 5s 刷出）：缓冲满即丢弃计数，
// 绝不反压业务；OpenSearch 出口见 storage/opensearch/logsink（按日索引）。
//
// 已有 zap/slog 等自有设施的服务仍可只实现 Logger 五方法接入库内组件
// （NewSlog 适配标准库 slog；业务用 zap 时建议直接用 New）。
package logx
