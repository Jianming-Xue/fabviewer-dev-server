package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/common-lib/common/errs"
	"github.com/fab-viewer/common-lib/common/paging"
	"github.com/fab-viewer/common-lib/common/tx"
)

const (
	// HeaderTxID 是事务 ID 的请求/响应头字段名（请求携带则透传，见 middleware.RuntimeContext）。
	HeaderTxID = "txId"
	// HeaderTxTime 是事务时间的请求头字段名，格式 yyyyMMDDHHmmss
	// （缺失/非法由中间件兜底服务器时间）。
	HeaderTxTime = "txTime"
	// HeaderTxName 是事务名称的请求头字段名。
	HeaderTxName = "txName"
	// HeaderUserName 是操作用户名的请求头字段名（同时注入 actor 审计上下文）。
	HeaderUserName = "userName"

	// CtxTx 是 gin.Context 中存放请求运行时上下文 tx.Tx 的 key，
	// 由 middleware.RuntimeContext 写入。
	CtxTx = "commonlib.tx"
)

// Response 是统一响应体 apiresponse，成功/失败同构（五字段恒序列化）：
//
//	{"code":200,"success":true,"errMsg":"","contents":{...},"txId":"..."}
//	{"code":404,"success":false,"errMsg":"...","contents":null,"txId":"..."}
//
// code 即 HTTP 状态码（数字）；txId 取自请求头 txId（未启用 RuntimeContext 中间件时为空）。
// 业务字符串错误码（errs.Code）与 Details 不出参，只用于内部映射与日志。
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
		TxID:    TxIDOf(c),
	}
}

// TxOf 从 gin.Context 取请求运行时上下文（未启用 RuntimeContext 中间件时为零值）。
func TxOf(c *gin.Context) tx.Tx {
	v, ok := c.Get(CtxTx)
	if !ok {
		return tx.Tx{}
	}
	t, _ := v.(tx.Tx)
	return t
}

// TxIDOf 从 gin.Context 取事务 ID，是 TxOf(c).ID 的语法糖
// （未启用 RuntimeContext 中间件时为空字符串）。
func TxIDOf(c *gin.Context) string {
	return TxOf(c).ID
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

// Fail 写回失败响应，与 middleware.ErrorHandler 使用同一套默认映射：
// *errs.Error 按码表映射；sql.ErrNoRows → 404；context.Canceled → 499；
// 其余 → 500 且根因不进响应。code 即映射出的 HTTP 状态码。
// 注意 gorm.ErrRecordNotFound 的翻译只在 middleware.ErrorHandler 中内置
// （避免 api 包引入 GORM 依赖）。
func Fail(c *gin.Context, err error) {
	if c.Writer.Written() {
		return
	}
	var e *errs.Error
	if errors.As(err, &e) {
		r := NewResponse(c, StatusFor(e.Code), e.Message)
		Write(c, r.Code, &r)
		return
	}
	switch {
	case errors.Is(err, sql.ErrNoRows):
		r := NewResponse(c, http.StatusNotFound, "resource not found")
		Write(c, http.StatusNotFound, &r)
	case errors.Is(err, context.Canceled):
		r := NewResponse(c, StatusClientClosedRequest, "client canceled request")
		Write(c, StatusClientClosedRequest, &r)
	default:
		r := NewResponse(c, http.StatusInternalServerError, "internal error")
		Write(c, http.StatusInternalServerError, &r)
	}
}
