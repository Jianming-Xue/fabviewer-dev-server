// Package api 是接口层：HTTP 路由与 handler，只认识 services 接口与
// common-lib 的 API 组件（统一响应/分页解析/中间件），不含业务逻辑。
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/common-lib/api/middleware"
	"github.com/fab-viewer/common-lib/common/bizerror"
	"github.com/fab-viewer/common-lib/common/logx"
	"github.com/fab-viewer/security-management/services"
	"github.com/fab-viewer/security-management/usererrors"
)

// NewRouter 组装路由与中间件栈。注册顺序固定（见 middleware 包文档）：
// AccessLog 最外层保证错误路径状态码是最终值。
func NewRouter(log logx.Logger, users services.UserService) *gin.Engine {
	r := gin.New()
	r.Use(
		middleware.AccessLog(log),
		middleware.ErrorHandler(log,
			// 模块错误码 → HTTP 状态码
			middleware.WithStatusMap(map[bizerror.Code]int{
				usererrors.CodeUserNotFound:      http.StatusNotFound,
				usererrors.CodeUserAlreadyExists: http.StatusConflict,
				usererrors.CodeConfirmDeleteUser: http.StatusConflict,
				usererrors.CodeInvalidUserID:     http.StatusBadRequest,
			}),
		),
		middleware.Recovery(log),
		middleware.FillRuntimeContext(),
	)

	r.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	g := r.Group("/api")
	registerUserRoutes(g, users)
	return r
}

// registerUserRoutes 注册用户模块路由。
func registerUserRoutes(g *gin.RouterGroup, users services.UserService) {
	h := &userHandler{users: users}
	g.POST("/users", h.create)
	g.GET("/users", h.list)
	g.GET("/users/:id", h.get)
	g.DELETE("/users/:id", h.delete)
}
