package usererrors

// MessagesEn 是本模块的英文（默认语言）错误文案：key 即错误 code，value
// 为文案模板（{0} {1} 为位置占位符，InsertArgs 按序替换）。文件名即语言
// （messages_<语言>.go），新增错误后重跑 go generate（见 doc.go）更新构造函数。
var MessagesEn = map[string]string{
	"InvalidUserID":     "invalid user id",
	"UserNotFound":      "user {0} not found",
	"ConfirmDeleteUser": "confirm deleting user {0}?",
}
