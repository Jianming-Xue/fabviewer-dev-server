// Package response 提供统一响应体 ApiResponse 的对象、写回与 VO → DTO 转换。
//
// 统一响应体契约（成功/失败同构，五字段恒序列化）：
//
//	{"code":200,"success":true,"errMsg":"","data":{...},"txId":"..."}
//	{"code":200,"success":false,"errMsg":"...","data":null,"txId":"..."}
//
// code 即 HTTP 状态码，txId 取自 RuntimeContext 记录的事务 ID；data 由
// 业务 handler 指定（single 对象或 multi 对象集合，分页 pageable 整体
// 作为 data）。出参时间/布尔字段转 wire 格式（"yyyy-MM-dd HH:mm:ss"
// 字符串、int 1/0）用 Wrap 或 units 原语。
//
// 典型 handler：
//
//	res, err := userRepo.List(c.Request.Context(), request.PageQuery(c))
//	if err != nil {
//		c.Error(err) // 不直接写响应，交给 errorhandler 统一处理
//		return
//	}
//	response.OK(c, res)
package response
