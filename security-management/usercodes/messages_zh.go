package usercodes

// MessagesZh 是用户模块的中文文案，key 与 MessagesEn 对齐（缺 key 回退
// 默认语言）；{0} 在句中位置可与英文不同。
var MessagesZh = map[string]string{
	"UserNotFound":      "用户 {0} 不存在",
	"UserAlreadyExists": "用户名 {0} 已存在",
	"InvalidUserID":     "参数 id 非法",
	"ConfirmDeleteUser": "确认删除用户 {0} 吗？",
}
