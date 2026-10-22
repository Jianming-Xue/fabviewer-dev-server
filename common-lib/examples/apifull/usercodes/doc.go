// Package usercodes 演示业务模块的码定义模式：文案按语言写成 Go 文件
// （messages_en.go/messages_zh.go，文件名即语言，key 即码，编译期直接
// 进包），messages_gen.go 由 i18nerr 生成器从这些文件生成——码常量与
// NewError/NewMessage 翻译入口，与文件 key 一一对齐，改文件后重跑
// go generate 即可。
//
//go:generate go run github.com/fab-viewer/common-lib/cmd/i18nerr -dir . -pkg usercodes -default en
package usercodes
