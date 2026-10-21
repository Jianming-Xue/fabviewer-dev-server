package usererrors

import (
	"net/http"

	"github.com/fab-viewer/common-lib/common/bizerror"
)

// CodeMap 返回本模块错误码 → 响应体业务 code 的映射（组装根
// provideRouter 与测试共用，经 server.NewEngine / errorhandler.WithCodeMap
// 生效，映射值进 ApiResponse.Code——HTTP 状态码恒 200；未映射的码按 0）。
// 值暂沿用原 HTTP 状态码数字（404/409/400），业务码段规划后统一调整。
func CodeMap() map[bizerror.Code]int {
	return map[bizerror.Code]int{
		CodeUserNotFound:      http.StatusNotFound,
		CodeUserAlreadyExists: http.StatusConflict,
		CodeConfirmDeleteUser: http.StatusConflict,
		CodeInvalidUserID:     http.StatusBadRequest,
	}
}
