// Package i18n 提供基于请求上下文语言的后端消息国际化能力。
//
// 本包只提供机制、不内置任何文案：消息 key 与各语言译文由各业务模块
// 自行定义（messages_<语言>.go 语言包，经 cmd/i18nerr 生成注册），默认
// 语言由各模块字典自行指定。
//
// 能力三件套：
//
//   - 语言解析：ParseAcceptLanguage 解析标准 Accept-Language 头（含 q 值
//     排序），middleware/enrich.Enrich 取首选语言规范化后注入
//     rctx.RuntimeContext 的请求语言
//   - 消息集：Bundles 按「语言 → key → 模板」组织译文（每模块一个字典，
//     编译期进包、无全局注册表，同 key 跨模块互不冲突），Register 注册，
//     T/TOf 按回退链查找（请求语言 → 基础语言 → 默认语言 → key 原文）
//   - 请求翻译：T(ctx, key, ...) 从请求上下文取语言，接口/业务/存储层
//     全链路可用（与 rctx 同模式）；InsertArgs 供拿到模板 string 后
//     自行替换占位符
//
// 推荐接入姿势（多语言 Go 文件，编译期进包，详见 cmd/i18nerr 与
// examples/apifull/usercodes）：
//
//	// 1. 模块码包内按语言写消息文件（messages_<语言>.go，key 即码）
//	var MessagesZh = map[string]string{"UserNotFound": "用户 {0} 不存在"}
//
//	// 2. 生成码常量与翻译入口（文件 key 与代码一一对齐）
//	//    cd usercodes && go run github.com/fab-viewer/common-lib/cmd/i18nerr \
//	//        -dir . -pkg usercodes -default en
//
//	// 3. 错误处与文案处直接用模块入口（按请求语言翻译）
//	usercodes.NewError(ctx, usercodes.UserNotFound).InsertArgs(id)
//	tip := usercodes.NewMessage(ctx, usercodes.OrderSubmitted, orderID)
package i18n
