// Package services 定义对外能力接口与出入参（DTO）：api 层只认识本包，
// app 层提供实现。接口即契约——错误语义（编码/消息/语言）由 app 层经
// fverr 表达，本包不涉及。
package services

import (
	"context"

	"github.com/fab-viewer/common-lib/common/paging"
)

// UserDTO 是用户的对外视图。
type UserDTO struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// CreateUserCmd 是创建用户入参。
type CreateUserCmd struct {
	Username string `json:"username" binding:"required"`
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email"`
	Enabled  bool   `json:"enabled"`
}

// UserService 是用户模块对外能力。Delete 的确认语义：confirmed=false 时
// 返回带用户名的确认消息（前端弹框），true 时真正删除。
type UserService interface {
	Create(ctx context.Context, cmd CreateUserCmd) (UserDTO, error)
	Get(ctx context.Context, id string) (UserDTO, error)
	List(ctx context.Context, q paging.Query) (paging.Pageable[UserDTO], error)
	Delete(ctx context.Context, id string, confirmed bool) error
}
