package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/common-lib/api/reqctx"
	"github.com/fab-viewer/common-lib/common/paging"
)

// Response 是统一响应体 apiresponse，成功/失败同构（五字段恒序列化）：
//
//	{"code":200,"success":true,"errMsg":"","contents":{...},"txId":"..."}
//	{"code":404,"success":false,"errMsg":"...","contents":null,"txId":"..."}
//
// code 即 HTTP 状态码（数字）；txId 取自请求头 txId（未启用
// runtimecontext.FillRuntimeContext 中间件时为空）。
// 业务字符串错误码（bizerror.Code）与 Details 不出参，只用于内部映射与日志。
type Response struct {
	Code     int    `json:"code"`
	Success  bool   `json:"success"`
	ErrMsg   string `json:"errMsg"`
	Contents any    `json:"contents"`
	TxID     string `json:"txId"`
}

// NewResponse 组装含事务 ID 的响应体：Success 按 status < 400 判定。
func NewResponse(c *gin.Context, status int, errMsg string) Response {
	return Response{
		Code:    status,
		Success: status < http.StatusBadRequest,
		ErrMsg:  errMsg,
		TxID:    reqctx.TxIDOf(c),
	}
}

// Write 将 Response 以 JSON 写回并设置 HTTP 状态码；响应已写过时跳过（防双写）。
func Write(c *gin.Context, status int, resp *Response) {
	if c.Writer.Written() {
		return
	}
	c.JSON(status, resp)
}

// OK 写回 200 成功响应。
func OK(c *gin.Context, contents any) {
	r := NewResponse(c, http.StatusOK, "")
	r.Contents = contents
	Write(c, http.StatusOK, &r)
}

// Created 写回 201 资源创建成功响应。
func Created(c *gin.Context, contents any) {
	r := NewResponse(c, http.StatusCreated, "")
	r.Contents = contents
	Write(c, http.StatusCreated, &r)
}

// NoContent 写回 204 无内容响应（无响应体）。
func NoContent(c *gin.Context) {
	if c.Writer.Written() {
		return
	}
	c.Status(http.StatusNoContent)
}

// OKPage 写回 200 分页成功响应，contents 即分页出参对象 pageable。
func OKPage[T any](c *gin.Context, p paging.Pageable[T]) {
	OK(c, p)
}
