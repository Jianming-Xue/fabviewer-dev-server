// Package errresp 提供错误码 → HTTP 状态码映射（bizerror.Code →
// response.ApiResponse.Code）：DefaultStatusMap/StatusFor，供
// errorhandler 中间件写回统一响应体时取状态码。业务模块错误码经
// errorhandler.WithStatusMap 增补映射；手动写回失败响应用
// response.Fail（本包不提供写回）。
package errresp
