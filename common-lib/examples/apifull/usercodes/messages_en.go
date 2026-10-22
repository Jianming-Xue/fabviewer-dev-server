package usercodes

// MessagesEn 是本模块的英文（默认语言）文案：key 即码，value 为文案
// 模板（{0} {1} 为位置占位符，按序替换）。文件名即语言
// （messages_<语言>.go），新增码后重跑 go generate（见 doc.go）。
var MessagesEn = map[string]string{
	"InvalidUserID":     "invalid user id",
	"UserNotFound":      "user {0} not found",
	"ConfirmDeleteUser": "confirm deleting user {0}?",
}
