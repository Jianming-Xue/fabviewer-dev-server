// Package usererrors 是用户模块的错误语言包：文案按语言写成 Go 文件
// （messages_<语言>.go，key 即 code），errors_gen.go 由 i18nerr 生成器产出
// （Code 常量、模块字典 ErrorCode、每 key 一个构造函数）。
//
//go:generate go run github.com/fab-viewer/common-lib/cmd/i18nerr -dir . -pkg usererrors -default en
package usererrors
