// Package api 提供 RESTful API 的公共组件：统一响应体、分页参数解析，
// 仅可被业务系统的接口访问层（及组装根 main）import。
//
// 统一响应体契约（成功/失败同构）：
//
//	{"code":"OK","message":"success","data":{...},"requestId":"...","timestamp":"..."}
//	{"code":"NOT_FOUND","message":"...","details":{...},"requestId":"...","timestamp":"..."}
//
// 典型 handler：
//
//	q := api.PageQuery(c)
//	res, err := userRepo.List(c.Request.Context(), q)
//	if err != nil {
//		c.Error(err) // 不直接写响应，交给 ErrorHandler 统一处理
//		return
//	}
//	api.OKPage(c, res)
//
// 中间件注册顺序固定（见 middleware 包文档）：
//
//	AccessLog → ErrorHandler → Recovery → RequestID → 路由
package api
