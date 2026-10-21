package errresp

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/common-lib/api/response"
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

// Fail 写回失败响应，与 errorhandler.ErrorHandler 使用同一套默认映射：
// *bizerror.BizError 按码表映射；sql.ErrNoRows → 404；context.Canceled → 499；
// 其余 → 500 且根因不进响应。code 即映射出的 HTTP 状态码。
// 注意 gorm.ErrRecordNotFound 的翻译只在 errorhandler.ErrorHandler 中内置
// （避免本包引入 GORM 依赖）。
func Fail(c *gin.Context, err error) {
	if c.Writer.Written() {
		return
	}
	var e *bizerror.BizError
	if errors.As(err, &e) {
		r := response.NewResponse(c, StatusFor(e.Code), e.Message)
		response.Write(c, r.Code, &r)
		return
	}
	switch {
	case errors.Is(err, sql.ErrNoRows):
		r := response.NewResponse(c, http.StatusNotFound, "resource not found")
		response.Write(c, http.StatusNotFound, &r)
	case errors.Is(err, context.Canceled):
		r := response.NewResponse(c, StatusClientClosedRequest, "client canceled request")
		response.Write(c, StatusClientClosedRequest, &r)
	default:
		r := response.NewResponse(c, http.StatusInternalServerError, "internal error")
		response.Write(c, http.StatusInternalServerError, &r)
	}
}
