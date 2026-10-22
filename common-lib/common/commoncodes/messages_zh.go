package commoncodes

// MessagesZh 是 common-lib 公共码的中文文案，key 与 MessagesEn 对齐
// （缺 key 回退默认语言英文）；{0} 在句中位置可与英文不同。
var MessagesZh = map[string]string{
	"ParameterRequired": "参数 {0} 必填",
	"InvalidParameter":  "参数 {0} 非法",
	"InvalidTime":       "参数 {0} 的时间值 {1} 非法，应为 yyyyMMdd / yyyyMMddHHmmss / 13 位毫秒时间戳",
	"InternalError":     "内部错误",
	"NotFound":          "未找到",
	"Unauthorized":      "未授权",
	"Forbidden":         "禁止访问",
	"Conflict":          "资源冲突",
	"RateLimited":       "请求过于频繁",
	"DBError":           "数据库访问错误",
}
