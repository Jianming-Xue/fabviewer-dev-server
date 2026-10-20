package api

import (
	"net/http"

	"github.com/fab-viewer/common-lib/common/fverr"
)

// StatusClientClosedRequest 表示客户端主动断开（nginx 惯例 499，非标准状态码）。
const StatusClientClosedRequest = 499

// DefaultStatusMap 是默认的错误码 → HTTP 状态码映射。
// 业务自定义错误码可经 middleware.WithStatusMap 增补或覆盖；
// 未映射的自定义码按 500 处理。
var DefaultStatusMap = map[fverr.Code]int{
	fverr.CodeOK:           http.StatusOK,
	fverr.CodeValidation:   http.StatusBadRequest,
	fverr.CodeUnauthorized: http.StatusUnauthorized,
	fverr.CodeForbidden:    http.StatusForbidden,
	fverr.CodeNotFound:     http.StatusNotFound,
	fverr.CodeConflict:     http.StatusConflict,
	fverr.CodeTooMany:      http.StatusTooManyRequests,
	fverr.CodeInternal:     http.StatusInternalServerError,
	fverr.CodeDB:           http.StatusInternalServerError,
	fverr.CodeUpstream:     http.StatusBadGateway,
}

// StatusFor 返回错误码对应的 HTTP 状态码，未映射的码返回 500。
func StatusFor(code fverr.Code) int {
	if s, ok := DefaultStatusMap[code]; ok {
		return s
	}
	return http.StatusInternalServerError
}
