// Package app 提供 services 包接口的实现：编排 domain 端口、表达业务
// 错误语义（经模块错误语言包 usererrors，按请求语言生成 Message）。
package app

import (
	"context"
	"time"

	"github.com/fab-viewer/common-lib/common/fverr"
	"github.com/fab-viewer/common-lib/common/paging"
	"github.com/fab-viewer/security-management/domain"
	"github.com/fab-viewer/security-management/services"
	"github.com/fab-viewer/security-management/usererrors"
)

// UserService 实现 services.UserService。
type UserService struct {
	users domain.UserRepository
}

// NewUserService 组装用户服务；users 为存储端口实现（oracle/memory）。
func NewUserService(users domain.UserRepository) *UserService {
	return &UserService{users: users}
}

// Create 创建用户：登录名重复返回 UserAlreadyExists；成功回查返回完整 DTO。
func (s *UserService) Create(ctx context.Context, cmd services.CreateUserCmd) (services.UserDTO, error) {
	if dup, err := s.users.FindByUsername(ctx, cmd.Username); err != nil {
		return services.UserDTO{}, wrapStorage(ctx, "FindByUsername", err)
	} else if dup != nil {
		return services.UserDTO{}, usererrors.NewUserAlreadyExists(ctx).
			InsertArgs(cmd.Username).
			AttachError(nil, map[string]any{"username": cmd.Username})
	}

	u := &domain.User{
		Username: cmd.Username,
		Name:     cmd.Name,
		Email:    cmd.Email,
		Enabled:  cmd.Enabled,
	}
	if err := s.users.Create(ctx, u); err != nil {
		return services.UserDTO{}, wrapStorage(ctx, "Create", err)
	}
	return s.Get(ctx, u.ID)
}

// Get 按 ID 查询，未找到返回 UserNotFound（带 ID 便于定位）。
func (s *UserService) Get(ctx context.Context, id string) (services.UserDTO, error) {
	u, err := s.users.FindByID(ctx, id)
	if err != nil {
		return services.UserDTO{}, wrapStorage(ctx, "FindByID", err)
	}
	if u == nil {
		return services.UserDTO{}, usererrors.NewUserNotFound(ctx).
			InsertArgs(id).
			AttachError(nil, map[string]any{"id": id})
	}
	return toDTO(u), nil
}

// List 分页查询。
func (s *UserService) List(ctx context.Context, q paging.Query) (paging.Pageable[services.UserDTO], error) {
	page, err := s.users.List(ctx, q)
	if err != nil {
		return paging.Pageable[services.UserDTO]{}, wrapStorage(ctx, "List", err)
	}
	dtos := make([]services.UserDTO, 0, len(page.Data))
	for i := range page.Data {
		dtos = append(dtos, toDTO(&page.Data[i]))
	}
	return paging.NewPageable(q, page.TotalSize, dtos), nil
}

// Delete 删除用户：confirmed=false 返回带显示名的确认消息（前端弹框后
// 带 confirm=true 重试）；true 时真正删除。
func (s *UserService) Delete(ctx context.Context, id string, confirmed bool) error {
	u, err := s.users.FindByID(ctx, id)
	if err != nil {
		return wrapStorage(ctx, "FindByID", err)
	}
	if u == nil {
		return usererrors.NewUserNotFound(ctx).
			InsertArgs(id).
			AttachError(nil, map[string]any{"id": id})
	}
	if !confirmed {
		// 确认消息必须带上被删对象的 Name（用户模块即显示名）
		return usererrors.NewConfirmDeleteUser(ctx).
			InsertArgs(u.Name).
			AttachError(nil, map[string]any{"id": id, "name": u.Name})
	}
	if err := s.users.Delete(ctx, id); err != nil {
		return wrapStorage(ctx, "Delete", err)
	}
	return nil
}

// wrapStorage 包装存储层技术错误：保留根因只进日志，Message 带操作名。
func wrapStorage(ctx context.Context, op string, err error) error {
	return fverr.NewError(fverr.CodeDB, "user storage op={0}", op).
		AttachError(err, nil)
}

// toDTO 实体转对外视图。
func toDTO(u *domain.User) services.UserDTO {
	return services.UserDTO{
		ID:        u.ID,
		Username:  u.Username,
		Name:      u.Name,
		Email:     u.Email,
		Enabled:   u.Enabled,
		CreatedAt: u.CreatedAt.Format(time.DateTime),
		UpdatedAt: u.UpdatedAt.Format(time.DateTime),
	}
}

var _ services.UserService = (*UserService)(nil)
