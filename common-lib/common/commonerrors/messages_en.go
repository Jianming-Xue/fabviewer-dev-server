package commonerrors

// MessagesEn 是 common-lib 公共错误的英文（默认语言）文案：key 即错误
// code（响应体业务 code，fault 直接透传），value 为文案模板（{0} {1} 为
// 位置占位符，InsertArgs 按序替换）。业务服务报错时可直接引用这些公共
// 码组织错误（common-lib 的 request 校验路径即用它们）。
var MessagesEn = map[string]string{
	"ParameterRequired": "parameter {0} is required",
	"InvalidParameter":  "invalid parameter {0}",
	"InvalidTime":       "parameter {0}: invalid time {1}, want yyyyMMdd / yyyyMMddHHmmss / 13-digit millis",
	"InternalError":     "internal error",
}
