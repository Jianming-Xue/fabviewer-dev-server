// Package config 提供 Oracle 基础连接配置：Config 值对象、WithDefaults
// 补默认、Validate 校验、BuildDSN 生成 godror 连接串（SessionParams 双层
// 引号转义）。纯数据与校验，不含 SDK 装配逻辑。
package config
