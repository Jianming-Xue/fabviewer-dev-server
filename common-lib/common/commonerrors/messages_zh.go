package commonerrors

// MessagesZh 是 common-lib 公共错误的中文文案，key 与 MessagesEn 对齐
// （缺 key 回退默认语言英文）；{0} 在句中位置可与英文不同。
var MessagesZh = map[string]string{
	"ParameterRequired": "参数 {0} 必填",
	"InvalidParameter":  "参数 {0} 非法",
	"InvalidTime":       "参数 {0} 的时间值 {1} 非法，应为 yyyyMMdd / yyyyMMddHHmmss / 13 位毫秒时间戳",
	"InternalError":     "内部错误",
}
