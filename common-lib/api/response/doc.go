// Package response 提供统一响应体 apiresponse 的参数对象与写回。
//
// 统一响应体契约（成功/失败同构，五字段恒序列化）：
//
//	{"code":200,"success":true,"errMsg":"","contents":{...},"txId":"..."}
//	{"code":404,"success":false,"errMsg":"...","contents":null,"txId":"..."}
//
// code 即 HTTP 状态码；txId 取自请求头 txId；分页出参 pageable
// （pageIndex/pageSize/totalSize/totalPage/data）置于 contents。
//
// 典型 handler：
//
//	q := request.PageQuery(c) // 解析 ?pageIndex=&pageSize=
//	res, err := userRepo.List(c.Request.Context(), q)
//	if err != nil {
//		c.Error(err) // 不直接写响应，交给 errorhandler 统一处理
//		return
//	}
//	response.OKPage(c, res)
package response
