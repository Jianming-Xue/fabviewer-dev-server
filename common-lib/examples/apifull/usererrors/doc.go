// Package usererrors 演示业务模块的错误定义模式：错误文案按语言写成 Go 文件
// （messages_en.go/messages_zh.go，文件名即语言，key 即错误变量名，编译期
// 直接进包），errors_gen.go 由 i18nerr 生成器从这些文件生成——ErrorCode 字典句柄、
// code 常量与构造函数，与文件 key 一一对齐，改文件后重跑 go generate 即可。
//
// 服务启动时调用 usererrors.RegisterAll(bundles) 一行注册全部语言（见 main.go）。
//
//go:generate go run github.com/fab-viewer/common-lib/cmd/i18nerr -dir . -pkg usererrors -default en
package usererrors
