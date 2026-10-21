// Package units 提供跨层通用的输入/输出转换原语：wire 约定格式与 Go
// 类型间的纯函数转换——布尔约定为 int（输入 1 → true 其余含 0 → false，
// 输出 true → 1 / false → 0）、日期时间输入为 yyyyMMdd / yyyyMMddHHmmss
// 字符串（与请求头 txTime 同族）、毫秒时间戳为 13 位数字串，出参时间
// 格式化为同族的 yyyyMMddHHmmss 字符串。零三方依赖，api 层与业务各层可直接调用。
package units
