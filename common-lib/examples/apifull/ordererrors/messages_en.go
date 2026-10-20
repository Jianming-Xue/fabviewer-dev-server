package ordererrors

// MessagesEn 是订单模块的英文（默认语言）错误文案，与 usererrors 模块的
// 语言包相互独立、启动时合并注册进同一消息集（见 main.go）。
var MessagesEn = map[string]string{
	"OrderNotFound": "order not found",
}
