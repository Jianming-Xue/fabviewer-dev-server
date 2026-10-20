package usererrors

// MessagesZh 是本模块的中文错误文案，key 与 MessagesEn 对齐（缺 key 时
// 回退默认语言文案）；占位符 {0} 在句中位置可与英文不同。
var MessagesZh = map[string]string{
	"InvalidUserID":     "参数 id 非法",
	"UserNotFound":      "用户 {0} 不存在",
	"ConfirmDeleteUser": "确认删除用户 {0} 吗？",
}
