// Package bizerror 提供 fab-viewer 体系的统一业务错误 BizError：全库唯一出门
// 错误类型，四字段（Code/Message/Err/Details），出口只输出 Message，
// 其余全部进日志。
//
// API 收敛为四个方法 + 少量配套（NewError 唯一构造，行为由参数与链式
// 调用调节）：
//
//	// 唯一构造：Message 直传，args 按序替换 {0} {1} 占位符
//	bizerror.NewError(code, message, args...)
//	    .InsertArgs(...)        // 追加插参（与构造 args 等价，按需选用）
//	    .AttachError(err, det)  // 补根因与明细（只进日志）
//	bizerror.IsError(err)          // 判断错误链中是否含 BizError
//	bizerror.From(err)             // 取出 *BizError（检查 Code 等）
//
// 业务层的多语言创建经 cmd/i18nerr 生成的模块入口（如
// usercodes.NewError(ctx, usercodes.UserNotFound)）：内部先按请求语言从
// 模块字典查模板，再交给 NewError，详见 cmd/i18nerr 与 examples/apifull。
//
// 本包零三方依赖。
package bizerror
