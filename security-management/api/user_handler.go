package api

import (
	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/common-lib/api/request"
	"github.com/fab-viewer/common-lib/api/response"
	"github.com/fab-viewer/security-management/services"
	"github.com/fab-viewer/security-management/usererrors"
)

// userHandler 用户模块 handler：入参校验、调 services、错误经 c.Error
// 交给 ErrorHandler（统一响应体/状态码/日志），不自写错误响应。
type userHandler struct {
	users services.UserService
}

// create POST /api/users
func (h *userHandler) create(c *gin.Context) {
	var cmd services.CreateUserCmd
	if err := c.ShouldBindJSON(&cmd); err != nil {
		_ = c.Error(usererrors.NewInvalidUserID(c.Request.Context()).
			AttachError(err, nil))
		return
	}
	dto, err := h.users.Create(c.Request.Context(), cmd)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Created(c, dto)
}

// list GET /api/users?pageIndex=&pageSize=
func (h *userHandler) list(c *gin.Context) {
	page, err := h.users.List(c.Request.Context(), request.PageQuery(c))
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OKPage(c, page)
}

// get GET /api/users/:id
func (h *userHandler) get(c *gin.Context) {
	dto, err := h.users.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.OK(c, dto)
}

// delete DELETE /api/users/:id?confirm=true
func (h *userHandler) delete(c *gin.Context) {
	if c.Param("id") == "" {
		_ = c.Error(usererrors.NewInvalidUserID(c.Request.Context()))
		return
	}
	if err := h.users.Delete(c.Request.Context(), c.Param("id"),
		c.Query("confirm") == "true"); err != nil {
		_ = c.Error(err)
		return
	}
	response.NoContent(c)
}
