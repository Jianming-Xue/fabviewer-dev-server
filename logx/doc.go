// Package logx 定义最小日志接口 Logger 及其 slog 适配器与空实现，
// 供库内组件（中间件、存储包装）与业务系统统一日志出口，不绑定具体日志库。
//
// 业务系统已有 zap/logrus 等日志设施时，实现本接口（五个方法）即可接入；
// 默认提供 NewSlog(*slog.Logger) 适配标准库 slog。库内组件通过 Default 或
// 注入的 Logger 输出日志，未注入时静默（Nop）。
//
// 本包零三方依赖（slog 为标准库）。
package logx
