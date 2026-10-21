// Package i18n 提供基于请求上下文语言的后端消息国际化能力。
//
// 本包只提供机制、不内置任何文案：消息 key 与各语言译文由各业务服务
// 自行定义注册（通常在组装根完成），默认语言也由业务服务指定。
//
// 能力三件套：
//
//   - 语言解析：ParseAcceptLanguage 解析标准 Accept-Language 头（含 q 值
//     排序），middleware.FillRuntimeContext 取首选语言规范化后
//     注入 rctx.RuntimeContext 的请求语言
//   - 消息集：Bundles 按「语言 → key → 模板」组织译文，Register 注册，
//     T/TOf 按回退链查找（请求语言 → 基础语言 → 默认语言 → key 兜底模板）
//   - 请求翻译：T(ctx, key, ...) 从请求上下文取语言，接口/业务/存储层
//     全链路可用（与 rctx 同模式）
//
// 推荐接入姿势（多语言 Go 文件，编译期进包，详见 cmd/i18nerr 与
// examples/apifull/usererrors）：
//
//	// 1. 业务模块内按语言写消息文件（messages_<语言>.go，key 即错误变量名）
//	var MessagesZh = map[string]string{"UserNotFound": "用户不存在"}
//
//	// 2. 生成错误变量集合与 RegisterAll（文件 key 与代码变量一一对齐）
//	//    cd usererrors && go run github.com/fab-viewer/common-lib/cmd/i18nerr \
//	//        -dir . -pkg usererrors -default en
//
//	// 3. 组装根注册全部语言、接通错误出口
//	usererrors.RegisterAll(bundles) // bundles := i18n.NewBundles(i18n.En)
//	r.Use(middleware.ErrorHandler(log, middleware.WithI18N(bundles)), ...)
//
//	// 4. 错误处直接抛变量；含动态参数的消息在业务层就地翻译
//	return usererrors.ErrUserNotFound
//	errs.Wrapf(errs.ErrNotFound, errs.CodeNotFound, "%s",
//	    bundles.T(ctx, string(usererrors.ErrUserNotFound.Code), id))
//
// key 未注册译文时按默认语言的 key 模板格式化兜底（默认语言通常取
// 开发语言，key 即兜底文案）。
package i18n
