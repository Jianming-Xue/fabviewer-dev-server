package bizerror

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Code 是面向调用方的字符串错误码，与响应体业务 code 同一命名空间。
// 本包零内置码：库级公共码在 common/commoncodes，业务模块码在各
// xxxcodes 码包（key 即码）。
type Code string

// BizError 是统一业务错误：Code 定语义、Message 面向调用方可展示、Err 保留
// 根因链（只进日志不进响应）、Details 携带结构化明细（只进日志）。
type BizError struct {
	Code    Code
	Message string
	Err     error
	Details map[string]any
}

// Error 实现 error 接口（编译要求，缺了 BizError 就不能当 error 用），
// 由 fmt/日志等隐式调用，格式为 "[CODE] message: root"（无根因时省略尾部）。
func (e *BizError) Error() string {
	msg := fmt.Sprintf("[%s] %s", e.Code, e.Message)
	if e.Err != nil {
		return msg + ": " + e.Err.Error()
	}
	return msg
}

// Unwrap 实现 errors.Is/As 的拆包协议（标准库隐式调用）：根因检查靠它穿透
// （如 batch 包 ctx 取消后调用方 errors.Is(err, context.Canceled) 仍可命中），
// 链式 BizError 的同码命中也依赖 Is+Unwrap 组合。
func (e *BizError) Unwrap() error { return e.Err }

// Is 实现 errors.Is 的比较协议（标准库隐式调用）：Code 相同即命中，即使
// 是两个不同实例（opensearch 等处的 errors.Is(err, bizerror.ErrXxx) 判断依赖此语义）。
func (e *BizError) Is(target error) bool {
	var t *BizError
	if errors.As(target, &t) {
		return e.Code == t.Code
	}
	return false
}

// NewError 是唯一的 BizError 构造函数：Message 直传（不查表），args 非空时
// 按调用序替换 message 中的 {0} {1} ... 位置占位符（同 InsertArgs 语义）。
// 两类典型用法——
//
//	基础设施层直构：bizerror.NewError(bizerror.CodeDB, "query orders id={0}", id)
//	    .AttachError(err, nil)
//	业务层查表翻译（经生成器产出的模块入口，如 usercodes.NewError）：
//	    usercodes.NewError(ctx, usercodes.UserNotFound)  → 详见 cmd/i18nerr
func NewError(code Code, message string, args ...any) *BizError {
	e := &BizError{Code: code, Message: message}
	if len(args) > 0 {
		e.Message = insertArgs(e.Message, args)
	}
	return e
}

// InsertArgs 按调用顺序逐个替换 Message 中的 {0} {1} ... 占位符
// （不同语言占位符在句中位置不同没关系，{n} 始终对应第 n 个参数）。
// 链式返回自身。
func (e *BizError) InsertArgs(args ...any) *BizError {
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
func (e *BizError) AttachError(err error, details map[string]any) *BizError {
	e.Err = err
	if len(details) > 0 {
		e.Details = details
	}
	return e
}

// IsError 判断 err 错误链中是否含 BizError。
func IsError(err error) bool {
	_, ok := From(err)
	return ok
}

// From 是 errors.As 的语法糖：从错误链中取出 *BizError（含 Code/Details）。
func From(err error) (*BizError, bool) {
	var e *BizError
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}
