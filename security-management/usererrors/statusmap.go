package usererrors

import (
	"net/http"

	"github.com/fab-viewer/common-lib/common/bizerror"
)

// StatusMap 返回本模块错误码 → HTTP 状态码映射（组装根 provideRouter 与
// 测试共用，经 server.NewEngine / errorhandler.WithStatusMap 生效；
// 未映射的自定义码按 500 处理）。
func StatusMap() map[bizerror.Code]int {
	return map[bizerror.Code]int{
		CodeUserNotFound:      http.StatusNotFound,
		CodeUserAlreadyExists: http.StatusConflict,
		CodeConfirmDeleteUser: http.StatusConflict,
		CodeInvalidUserID:     http.StatusBadRequest,
	}
}
