package logx

// 本文件是业务侧记录日志的正面入口：Init 落位全局出口后，业务代码经
// 包级函数直接记录，不需要持有任何日志对象；库内机制（全局出口的注入
// 与读取、Logger 适配）见 logx.go，除测试经 SetLogger 注入外业务不应
// 依赖。
//
// 直调 zap 实例（而非经 Default() 接口中转）是为了让日志的 caller
// （zap AddCallerSkip(1) 跳过包装层后）指向业务调用行而非本包内部行。

// Debug 经全局出口记录 debug 级日志；未初始化时静默丢弃。
func Debug(msg string, kv ...any) {
	if s := sharedSugar(); s != nil {
		s.Debugw(msg, kv...)
		return
	}
	Default().Debug(msg, kv...)
}

// Info 经全局出口记录 info 级日志；未初始化时静默丢弃。
func Info(msg string, kv ...any) {
	if s := sharedSugar(); s != nil {
		s.Infow(msg, kv...)
		return
	}
	Default().Info(msg, kv...)
}

// Warn 经全局出口记录 warn 级日志；未初始化时静默丢弃。
func Warn(msg string, kv ...any) {
	if s := sharedSugar(); s != nil {
		s.Warnw(msg, kv...)
		return
	}
	Default().Warn(msg, kv...)
}

// Error 经全局出口记录 error 级日志；未初始化时静默丢弃。
func Error(msg string, kv ...any) {
	if s := sharedSugar(); s != nil {
		s.Errorw(msg, kv...)
		return
	}
	Default().Error(msg, kv...)
}
