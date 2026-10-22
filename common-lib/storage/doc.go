// Package storage 汇集各存储访问子包（oracle / opensearch）的公共类型，
// 当前提供连接池配置 PoolConfig。仅可被业务系统的存储层（及组装根 main）import。
//
// 各子包互不依赖，业务系统按需引入对应存储，未 import 的存储依赖不参与编译链接。
package storage
