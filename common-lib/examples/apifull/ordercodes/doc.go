// Package ordercodes 演示多模块各自的码包：与 usercodes 模块互不感知、
// 各持字典（同 key 跨模块互不冲突），接入方式完全一致（见 usercodes/doc.go）。
//
//go:generate go run github.com/fab-viewer/common-lib/cmd/i18nerr -dir . -pkg ordercodes -default en
package ordercodes
