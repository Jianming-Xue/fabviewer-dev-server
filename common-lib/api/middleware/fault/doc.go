// Package fault 提供统一错误出口中间件：handler 中经 c.Error(err) 上报的
// 错误（不区分 BizError 与否）在 c.Next() 返回后统一消费——记录失败日志
// 与调用栈，并经 response.Fail 写回错误响应（errMsg 直取 err.Error()，
// 出口不做翻译或固定文案替换；code 透传 BizError 的 bizerror.Code 字符串）。
//
// 必须注册在 observer 内层、guard 外层（见 api/middleware 包文档的顺序契约）。
package fault
