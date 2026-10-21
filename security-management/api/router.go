// Package api 是接口层：HTTP 路由与 handler，只认识 services 接口与
// common-lib 的 API 组件（统一响应/分页解析/中间件），不含业务逻辑。
// 引擎装配（四中间件固定顺序 + /healthz 探活）归 common-lib 的
// cmd/server.NewEngine，本包只注册业务路由。
package api

import (
	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/security-management/services"
)

// RegisterRoutes 在引擎上注册业务路由（业务路由组 /api）。
func RegisterRoutes(r *gin.Engine, users services.UserService) {
	g := r.Group("/api")
	registerUserRoutes(g, users)
}

// registerUserRoutes 注册用户模块路由。
func registerUserRoutes(g *gin.RouterGroup, users services.UserService) {
	h := &userHandler{users: users}
	g.POST("/users", h.create)
	g.GET("/users", h.list)
	g.GET("/users/:id", h.get)
	g.DELETE("/users/:id", h.delete)
}
