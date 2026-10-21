// Package errresp 提供错误路径的响应写回与错误码 → HTTP 状态码映射：
// DefaultStatusMap/StatusFor（bizerror.Code → HTTP 状态码）与 Fail
// （未挂 errorhandler 中间件的轻量场景直接写回失败响应）。
//
// 常规业务错误一律 c.Error(err) 交给 errorhandler.ErrorHandler 统一处理
// （含 gorm.ErrRecordNotFound 翻译）；业务模块错误码经
// errorhandler.WithStatusMap 增补映射。
package errresp
