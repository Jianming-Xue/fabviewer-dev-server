package errs

import (
	"errors"
	"fmt"
)

// Code 是面向调用方的字符串错误码，原样进入 HTTP 响应体。
type Code string

// 库级通用错误码。业务系统可自行扩展，建议自带业务前缀，例如：
//
//	const CodeUserDisabled errs.Code = "USER_DISABLED"
const (
	CodeOK           Code = "OK"
	CodeInternal     Code = "INTERNAL_ERROR"
	CodeValidation   Code = "VALIDATION_ERROR"
	CodeNotFound     Code = "NOT_FOUND"
	CodeUnauthorized Code = "UNAUTHORIZED"
	CodeForbidden    Code = "FORBIDDEN"
	CodeConflict     Code = "CONFLICT"
	CodeTooMany      Code = "RATE_LIMITED"
	CodeDB           Code = "DB_ERROR"
	CodeUpstream     Code = "UPSTREAM_ERROR"
)

// Error 是统一业务错误：Code 定语义、Message 面向调用方可展示、
// Err 保留根因链（只进日志不进响应）、Details 携带结构化明细。
type Error struct {
	Code    Code
	Message string
	Err     error
	Details map[string]any
}

// Error 实现 error 接口，格式为 "[CODE] message: root"（无根因时省略尾部）。
func (e *Error) Error() string {
	msg := fmt.Sprintf("[%s] %s", e.Code, e.Message)
	if e.Err != nil {
		return msg + ": " + e.Err.Error()
	}
	return msg
}

// Unwrap 保留根因链，使 errors.Is/As 可穿透本错误。
func (e *Error) Unwrap() error { return e.Err }

// Is 使 errors.Is(err, errs.ErrXxx) 在 Code 相同即命中（即使根因不同）。
func (e *Error) Is(target error) bool {
	var t *Error
	if errors.As(target, &t) {
		return e.Code == t.Code
	}
	return false
}

// WithDetail 链式附加一条明细，懒初始化 Details。
func (e *Error) WithDetail(key string, value any) *Error {
	if e.Details == nil {
		e.Details = make(map[string]any)
	}
	e.Details[key] = value
	return e
}

// New 构造不含根因的业务错误。
func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Newf 按格式构造不含根因的业务错误。
func Newf(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Wrap 包装根因错误。message 应包含操作名与关键参数（如 "get user id=42"）；
// 显式传入的 code 覆盖底层 *Error 的 Code——外层最了解当前语义，根因仍可经
// errors.Is/As 穿透访问。err 为 nil 时返回 nil，便于 return errs.Wrap(err, ...) 直透。
func Wrap(err error, code Code, message string) *Error {
	if err == nil {
		return nil
	}
	return &Error{Code: code, Message: message, Err: err}
}

// Wrapf 是 Wrap 的格式化版本。
func Wrapf(err error, code Code, format string, args ...any) *Error {
	if err == nil {
		return nil
	}
	return &Error{Code: code, Message: fmt.Sprintf(format, args...), Err: err}
}

// From 是 errors.As 的语法糖：从错误链中取出 *Error（含 Code/Details）。
func From(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// 库级 sentinel 错误，业务代码统一用 errors.Is 判断：
//
//	if errors.Is(err, errs.ErrNotFound) { ... }
var (
	ErrInternal     = New(CodeInternal, "internal error")
	ErrValidation   = New(CodeValidation, "validation failed")
	ErrNotFound     = New(CodeNotFound, "resource not found")
	ErrUnauthorized = New(CodeUnauthorized, "unauthorized")
	ErrForbidden    = New(CodeForbidden, "forbidden")
	ErrConflict     = New(CodeConflict, "conflict")
	ErrTooMany      = New(CodeTooMany, "too many requests")
	ErrDB           = New(CodeDB, "database error")
	ErrUpstream     = New(CodeUpstream, "upstream error")
)
