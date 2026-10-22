// Package usercodes 是用户模块的码包：文案按语言写成 Go 文件
// （messages_<语言>.go，key 即码），messages_gen.go 由 i18nerr 生成器产出
// （码常量、NewError/NewMessage 翻译入口）。
//
//go:generate go run github.com/fab-viewer/common-lib/cmd/i18nerr -dir . -pkg usercodes -default en
package usercodes
