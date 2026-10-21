package errresp

import (
	"net/http"

	"github.com/fab-viewer/common-lib/common/bizerror"
)

// StatusClientClosedRequest 表示客户端主动断开（nginx 惯例 499，非标准状态码）。
const StatusClientClosedRequest = 499

// DefaultStatusMap 是默认的错误码 → HTTP 状态码映射。
// 业务自定义错误码可经 errorhandler.WithStatusMap 增补或覆盖；
// 未映射的自定义码按 500 处理。
var DefaultStatusMap = map[bizerror.Code]int{
	bizerror.CodeOK:           http.StatusOK,
	bizerror.CodeValidation:   http.StatusBadRequest,
	bizerror.CodeUnauthorized: http.StatusUnauthorized,
	bizerror.CodeForbidden:    http.StatusForbidden,
	bizerror.CodeNotFound:     http.StatusNotFound,
	bizerror.CodeConflict:     http.StatusConflict,
	bizerror.CodeTooMany:      http.StatusTooManyRequests,
	bizerror.CodeInternal:     http.StatusInternalServerError,
	bizerror.CodeDB:           http.StatusInternalServerError,
	bizerror.CodeUpstream:     http.StatusBadGateway,
}

// StatusFor 返回错误码对应的 HTTP 状态码，未映射的码返回 500。
func StatusFor(code bizerror.Code) int {
	if s, ok := DefaultStatusMap[code]; ok {
		return s
	}
	return http.StatusInternalServerError
}
