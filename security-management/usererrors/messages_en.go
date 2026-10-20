package usererrors

// MessagesEn 是用户模块的英文（默认语言）错误文案：key 即错误 code，value
// 为文案模板（{0} {1} 为位置占位符，InsertArgs 按序替换）。
var MessagesEn = map[string]string{
	"UserNotFound":      "user {0} not found",
	"UserAlreadyExists": "username {0} already exists",
	"InvalidUserID":     "invalid user id",
	"ConfirmDeleteUser": "confirm deleting user {0}?",
}
