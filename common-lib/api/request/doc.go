// Package request 提供请求参数对象的解析、校验与包装：
//
//   - PageQuery：查询参数 → paging.PageRequest（绑定失败一律回退默认值）
//   - Require：必填字符串校验（去空白，空则 VALIDATION 错误）
//   - Time：时间字符串参数 → time.Time（8 位日期 / 14 位日期时间 / 13 位
//     毫秒按长度识别；空串按可选参数返回零值，非法报 VALIDATION）
//   - Wrap[VO]：api 层 DTO → service 层 VO（字段名对齐自动拷贝 + 弱类型
//     转换；布尔输入按 units.Bool 约定，时间字符串字段自动解析为
//     time.Time）
//
// 纯转换原语（布尔/日期时间）在 common/units，本包只做面向 handler 的组合。
package request
