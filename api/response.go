package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/common-lib/errs"
	"github.com/fab-viewer/common-lib/paging"
)

const (
	// HeaderRequestID 是请求 ID 的请求/响应头字段名。
	HeaderRequestID = "X-Request-Id"

	// CtxRequestID 是 gin.Context 中存放请求 ID 的 key，
	// 由 middleware.RequestID 写入。
	CtxRequestID = "commonlib.request_id"
)

// Body 是统一响应体，成功/失败同构：
//
//	{"code":"OK","message":"success","data":{...},"requestId":"...","timestamp":"..."}
//	{"code":"NOT_FOUND","message":"...","details":{...},"requestId":"...","timestamp":"..."}
type Body struct {
	Code      errs.Code      `json:"code"`
	Message   string         `json:"message"`
	Data      any            `json:"data,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
	RequestID string         `json:"requestId,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// NewBody 组装含请求 ID 与时间戳的响应体。
func NewBody(c *gin.Context, code errs.Code, message string) Body {
	return Body{
		Code:      code,
		Message:   message,
		RequestID: RequestIDOf(c),
		Timestamp: time.Now(),
	}
}

// RequestIDOf 从 gin.Context 取请求 ID（未启用 RequestID 中间件时为空字符串）。
func RequestIDOf(c *gin.Context) string {
	v, ok := c.Get(CtxRequestID)
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// Write 将 Body 以 JSON 写回并设置 HTTP 状态码；响应已写过时跳过（防双写）。
func Write(c *gin.Context, status int, body *Body) {
	if c.Writer.Written() {
		return
	}
	c.JSON(status, body)
}

// OK 写回 200 成功响应。
func OK(c *gin.Context, data any) {
	b := NewBody(c, errs.CodeOK, "success")
	b.Data = data
	Write(c, http.StatusOK, &b)
}

// Created 写回 201 资源创建成功响应。
func Created(c *gin.Context, data any) {
	b := NewBody(c, errs.CodeOK, "created")
	b.Data = data
	Write(c, http.StatusCreated, &b)
}

// NoContent 写回 204 无内容响应（无响应体）。
func NoContent(c *gin.Context) {
	if c.Writer.Written() {
		return
	}
	c.Status(http.StatusNoContent)
}

// OKPage 写回 200 分页成功响应，data 即分页结果集。
func OKPage[T any](c *gin.Context, p paging.Result[T]) {
	OK(c, p)
}

// Fail 写回失败响应，与 middleware.ErrorHandler 使用同一套默认映射：
// *errs.Error 按码表映射；sql.ErrNoRows → 404；context.Canceled → 499；
// 其余 → 500 且根因不进响应。注意 gorm.ErrRecordNotFound 的翻译只在
// middleware.ErrorHandler 中内置（避免 api 包引入 GORM 依赖）。
func Fail(c *gin.Context, err error) {
	if c.Writer.Written() {
		return
	}
	var e *errs.Error
	if errors.As(err, &e) {
		b := NewBody(c, e.Code, e.Message)
		b.Details = e.Details
		Write(c, StatusFor(e.Code), &b)
		return
	}
	switch {
	case errors.Is(err, sql.ErrNoRows):
		b := NewBody(c, errs.CodeNotFound, "resource not found")
		Write(c, http.StatusNotFound, &b)
	case errors.Is(err, context.Canceled):
		b := NewBody(c, errs.CodeInternal, "client canceled request")
		Write(c, StatusClientClosedRequest, &b)
	default:
		b := NewBody(c, errs.CodeInternal, "internal error")
		Write(c, http.StatusInternalServerError, &b)
	}
}
