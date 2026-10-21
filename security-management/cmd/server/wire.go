//go:build wireinject
// +build wireinject

package main

import (
	"net/http"

	"github.com/google/wire"

	server "github.com/fab-viewer/common-lib/cmd/server"
	"github.com/fab-viewer/security-management/app"
	"github.com/fab-viewer/security-management/services"
)

// InitializeServer 编译期装配全部组件（配置 map → 日志 → server 段 →
// 存储 → 服务 → 引擎/路由 → http.Server）。依赖图变化后重跑 wire：
//
//	wire ./cmd/server
//
// 忘了重跑会在编译期暴露（wire_gen.go 与 wire.go 签名不一致）。
func InitializeServer() (*http.Server, func(), error) {
	wire.Build(
		server.ProviderSet,
		wire.Value(server.AppName("security")),
		provideStorage,
		provideUserRepo,
		provideRouter,
		app.NewUserService,
		wire.Bind(new(services.UserService), new(*app.UserService)),
	)
	return nil, nil, nil
}
