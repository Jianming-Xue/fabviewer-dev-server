// Package server 是业务服务组装根的全家桶入口：统一引擎装配（四中间件
// 固定顺序 + /healthz 探活）、server 配置段、http.Server 与阻塞运行，
// 以及 wire 编译期装配的 ProviderSet。业务服务的 cmd/<svc> 经 wire 引用，
// main 保持纯粹：InitializeServer → Run。
//
// 典型用法（业务服务两步）：
//
//  1. cmd/<svc>/wire.go 引用公共 provider（engine 归业务，一行传入自己的
//     模块错误码表并注册路由）：
//
//     func InitializeServer() (*http.Server, func(), error) {
//     wire.Build(
//     server.ProviderSet,
//     wire.Value(server.AppName("security")),
//     provideRouter, app.NewUserService, provideStorage,
//     )
//     return nil, nil, nil
//     }
//
//     func provideRouter(users services.UserService) *gin.Engine {
//     r := server.NewEngine(usererrors.CodeMap())
//     api.RegisterRoutes(r, users)
//     return r
//     }
//
//  2. main.go 只剩装配与运行：
//
//     srv, cleanup, err := InitializeServer()
//     if err != nil { ... }
//     if err := server.Run(srv); err != nil { ... } // 信号后返回，cleanup 链优雅停机
//
// 不用 wire 也可直接调用 NewEngine/Init/NewServer/Run 手工装配
// （examples/apifull 即此形态）。
package server
