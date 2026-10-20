package fverr

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Code 是面向调用方的字符串错误码：语义分类 + 状态码映射键。
// 业务模块的 code 即模块错误文件中的 key（如 "UserNotFound"）。
type Code string

// 库级通用错误码（状态码映射表 api.DefaultStatusMap 的键）。
// 业务模块自带模块内 key，建议命名足够具体。
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

// FvError 是统一业务错误：Code 定语义、Message 面向调用方可展示、Err 保留
// 根因链（只进日志不进响应）、Details 携带结构化明细（只进日志）。
type FvError struct {
	Code    Code
	Message string
	Err     error
	Details map[string]any
}

// Error 实现 error 接口（编译要求，缺了 FvError 就不能当 error 用），
// 由 fmt/日志等隐式调用，格式为 "[CODE] message: root"（无根因时省略尾部）。
func (e *FvError) Error() string {
	msg := fmt.Sprintf("[%s] %s", e.Code, e.Message)
	if e.Err != nil {
		return msg + ": " + e.Err.Error()
	}
	return msg
}

// Unwrap 实现 errors.Is/As 的拆包协议（标准库隐式调用）：根因检查靠它穿透
// （如 batch 包 ctx 取消后调用方 errors.Is(err, context.Canceled) 仍可命中），
// 链式 FvError 的同码命中也依赖 Is+Unwrap 组合。
func (e *FvError) Unwrap() error { return e.Err }

// Is 实现 errors.Is 的比较协议（标准库隐式调用）：Code 相同即命中，即使
// 是两个不同实例（opensearch 等处的 errors.Is(err, fverr.ErrXxx) 判断依赖此语义）。
func (e *FvError) Is(target error) bool {
	var t *FvError
	if errors.As(target, &t) {
		return e.Code == t.Code
	}
	return false
}

// NewError 是唯一的 FvError 构造函数：Message 直传（不查表），args 非空时
// 按调用序替换 message 中的 {0} {1} ... 位置占位符（同 InsertArgs 语义）。
// 两类典型用法——
//
//	基础设施层直构：fverr.NewError(fverr.CodeDB, "query orders id={0}", id)
//	    .AttachError(err, nil)
//	业务层查表翻译（经生成器产出的模块构造函数，如 usererrors.NewUserNotFound）：
//	    fverr.NewError(code, 模块字典.T(ctx, string(code)))  → 详见 cmd/i18nerr
func NewError(code Code, message string, args ...any) *FvError {
	e := &FvError{Code: code, Message: message}
	if len(args) > 0 {
		e.Message = insertArgs(e.Message, args)
	}
	return e
}

// InsertArgs 按调用顺序逐个替换 Message 中的 {0} {1} ... 占位符
// （不同语言占位符在句中位置不同没关系，{n} 始终对应第 n 个参数）。
// 链式返回自身。
func (e *FvError) InsertArgs(args ...any) *FvError {
	e.Message = insertArgs(e.Message, args)
	return e
}

// insertArgs 将模板中的 {0} {1} ... 按序替换为参数值（无占位符或参数
// 多余时忽略）。i18n 包因依赖隔离各有一份同语义实现。
func insertArgs(template string, args []any) string {
	for i, a := range args {
		template = strings.ReplaceAll(template,
			"{"+strconv.Itoa(i)+"}", fmt.Sprint(a))
	}
	return template
}

// AttachError 补上根因与结构化明细（均可为 nil），链式返回自身。
// 根因与明细只进日志不进响应。
func (e *FvError) AttachError(err error, details map[string]any) *FvError {
	e.Err = err
	if len(details) > 0 {
		e.Details = details
	}
	return e
}

// IsError 判断 err 错误链中是否含 FvError。
func IsError(err error) bool {
	_, ok := From(err)
	return ok
}

// From 是 errors.As 的语法糖：从错误链中取出 *FvError（含 Code/Details）。
func From(err error) (*FvError, bool) {
	var e *FvError
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// 库级 sentinel 错误，供 errors.Is 判断（业务模块错误走各自的 code）：
//
//	if errors.Is(err, fverr.ErrNotFound) { ... }
//
// 仅保留有真实调用点的码；需要新 sentinel 时一行即可补回。
var (
	ErrValidation   = NewError(CodeValidation, "validation failed")
	ErrNotFound     = NewError(CodeNotFound, "resource not found")
	ErrUnauthorized = NewError(CodeUnauthorized, "unauthorized")
	ErrTooMany      = NewError(CodeTooMany, "too many requests")
	ErrDB           = NewError(CodeDB, "database error")
)
