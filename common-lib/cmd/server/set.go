package server

import (
	"github.com/google/wire"

	"github.com/fab-viewer/common-lib/common/config"
	"github.com/fab-viewer/common-lib/common/logx"
)

// AppName 是业务服务名：决定配置文件定位与叶子环境变量覆盖前缀
// （如 "security" → {appName}_CFG 环境变量、security.log.level 式覆盖，
// 见 common/config 包文档）。
type AppName string

// ProviderSet 提供 HTTP 服务装配的公共 provider：配置 map、日志、
// server 段（含 gin 模式设置）与 http.Server（cleanup 承载优雅停机）。
// engine 不在本集合——它归业务：一行调用 NewEngine 传自己的模块错误码表
// 并注册路由（见包文档），wire 会把它与本集合的 NewServer 连起来。
var ProviderSet = wire.NewSet(
	ProvideConfigMap,
	logx.Init,
	Init,
	NewServer,
)

// ProvideConfigMap 按业务服务名加载配置 map。
func ProvideConfigMap(name AppName) (map[string]any, error) {
	return config.LoadConfig(string(name))
}
