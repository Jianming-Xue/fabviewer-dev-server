// Package errs 提供统一的业务错误码体系：Code 错误码、Error 错误类型、
// sentinel 错误与 Wrap 包装函数，供全部业务层使用。
//
// 错误响应形如 {"code":"NOT_FOUND","message":"..."}，与 api 包的统一响应体对齐。
// 服务层/聚合层用 New/Wrap 构造，接口层由 api/middleware.ErrorHandler 消费并映射
// HTTP 状态码，存储层将底层驱动错误翻译为 CodeDB 等码。业务系统可自行扩展：
//
//	const CodeUserDisabled errs.Code = "USER_DISABLED"
//
// 本包零三方依赖，可被任意层安全 import。
package errs
