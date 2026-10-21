// Package api 提供 RESTful API 的公共组件：统一响应体、分页参数解析，
// 仅可被业务系统的接口访问层（及组装根 main）import。
//
// 统一响应体契约 apiresponse（成功/失败同构，五字段恒序列化）：
//
//	{"code":200,"success":true,"errMsg":"","contents":{...},"txId":"..."}
//	{"code":404,"success":false,"errMsg":"...","contents":null,"txId":"..."}
//
// code 即 HTTP 状态码；txId 取自请求头 txId；分页出参 pageable
// （pageSize/pageIndex/totalSize/data）置于 contents。
//
// 典型 handler：
//
//	q := api.PageQuery(c) // 解析 ?pageIndex=&pageSize=
//	res, err := userRepo.List(c.Request.Context(), q)
//	if err != nil {
//		c.Error(err) // 不直接写响应，交给 ErrorHandler 统一处理
//		return
//	}
//	api.OKPage(c, res)
//
// 中间件注册顺序固定（见 middleware 包文档）：
//
//	AccessLog → ErrorHandler → Recovery → FillRuntimeContext → 路由
//
// FillRuntimeContext 解析请求头 txId/txTime/txName/userName 与标准头
// Accept-Language 组装为 rctx.RuntimeContext 注入请求上下文（业务层经
// rctx.GetRuntimeContext 读取），
// userName 即审计操作者，Lang 供 common/i18n 按请求语言翻译。
package api
