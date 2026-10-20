// Package ordererrors 演示多模块各自的错误语言包：与 usererrors 模块互不
// 感知，各自维护 messages_<语言>.go 与生成的变量集合，组装根依次调用
// RegisterAll 合并注册进同一消息集（key 在服务内唯一即可）。
//
//go:generate go run github.com/fab-viewer/common-lib/cmd/i18nerr -dir . -pkg ordererrors -default en
package ordererrors
