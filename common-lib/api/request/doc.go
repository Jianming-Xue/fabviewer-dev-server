// Package request 提供请求参数对象的解析：查询参数 → common/paging 的请求
// 对象等。绑定失败（缺失、非数字等）一律回退默认值，handler 不感知绑定错误。
package request
