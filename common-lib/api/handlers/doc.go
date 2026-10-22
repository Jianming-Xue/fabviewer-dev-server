// Package handlers 提供 common-lib 内置的公共 handler 集合（探活等），
// 由 server.NewEngine 统一注册，业务也可单独引用挂载。按 handler 一个
// 文件追加（readyz_handler.go、后续的 livez_handler.go 等）。
package handlers
