// Package errorhandler 提供统一错误处理中间件：handler 中经 c.Error(err)
// 上报的错误在 c.Next() 返回后统一消费并写回统一响应体。
//
// 处理规则：*bizerror.BizError 直出 Message（创建时已翻译）并按码表映射
// 状态码；gorm.ErrRecordNotFound / sql.ErrNoRows → 404；context.Canceled
// → 499；其余 → 500 且根因只进日志。业务模块错误码经 WithStatusMap 增补。
//
// 必须注册在 accesslog 内层、recovery 外层（见 api/middleware 包文档的
// 顺序契约）。
package errorhandler
