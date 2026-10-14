// Package opensearch 提供 OpenSearch 访问封装（基于 opensearch-go/v4 官方 SDK）：
// 连接装配、泛型搜索 Search[T]、文档 Index/Get/Delete，仅可被业务系统的存储层
// （及组装根 main）import。
//
// SDK 类型被隔离在包内，经 SDK() 逃生舱暴露完整原生能力；升级 SDK 版本时业务
// 代码不需要改动。搜索直接消费 paging.Query（分页语义的跨层会合点）。
package opensearch
