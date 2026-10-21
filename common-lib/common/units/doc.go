// Package units 提供跨层通用的输入转换原语：wire 约定格式到 Go 类型的
// 纯函数转换——布尔输入约定为 int（1 → true，其余含 0 → false，零值即
// 默认 false）、日期时间输入约定为 yyyyMMdd / yyyyMMddHHmmss 字符串
// （与请求头 txTime 同族）。零三方依赖，api/request 与业务各层可直接调用。
package units
