// Package logx 提供完整日志工具：zap 引擎 + console/file 双出口（文件
// 滚动由 lumberjack 承担），并保留最小 Logger 接口作为库内组件（中间件、
// 存储包装）与业务系统的统一接缝。
//
// 组件自治初始化：logx.Init(configMap) 读取配置中 log 段（配置结构私有，
// 不对外暴露），成功后自动落位全局出口，业务代码经包级函数直接记录，
// 不需要持有任何日志对象：
//
//	log, closeLog, err := logx.Init(cfgMap) // 由 cmd/server 统一装配调用
//	if err != nil { ... }
//	defer closeLog()                        // 优雅停机处调用：Sync 并关闭文件
//	logx.Info("user created", "id", id)
//
// log 段形态（等级链：出口 level → log.level → info 静默兜底；format 取
// json（默认）|console）：
//
//	log:
//	  level: info
//	  console:
//	    enabled: true
//	    level: ""            # 空 → 用默认等级
//	    format: console
//	  file:
//	    enabled: true
//	    format: json
//	    path: systemlogs/server.log
//	    maxSize: 100         # 兆
//	    maxBackups: 100
//	    maxAge: 30           # 天
//	    compress: false
//	    localTime: true      # 备份文件名时间戳用本地时区，false 用 UTC
//
// 两个出口的 enabled 均为 false（或缺失 log 段）时 Init 返回
// ErrNoLogComponent。测试可经 SetLogger 注入（Nop 静默或捕获式 fake）；
// 已有 slog 设施的服务实现 Logger 五方法即可接入（NewSlog 适配）。
package logx
