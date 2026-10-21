// Package memory 提供 domain 端口的内存实现：开发自举（无数据库时启动
// 服务联调 API/i18n 全链路）与测试替身。生产装配走 storage/oracle。
package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/fab-viewer/common-lib/common/paging"
	"github.com/fab-viewer/common-lib/common/rctx"
	"github.com/fab-viewer/common-lib/storage/snowflake"
	"github.com/fab-viewer/security-management/domain"
)

// UserRepo 内存实现 domain.UserRepository：读多写少，读写锁足够。
type UserRepo struct {
	mu    sync.RWMutex
	byID  map[string]*domain.User
	clock func() time.Time
}

// NewUserRepo 创建内存用户仓储。
func NewUserRepo() *UserRepo {
	return &UserRepo{byID: map[string]*domain.User{}, clock: time.Now}
}

// Create 生成雪花 ID 与审计字段（操作者取运行时上下文）后写入。
func (r *UserRepo) Create(ctx context.Context, u *domain.User) error {
	now := r.clock()
	who := rctx.GetRuntimeContext(ctx).FromUser()
	r.mu.Lock()
	defer r.mu.Unlock()
	if u.ID == "" {
		u.ID = snowflake.Next()
	}
	u.CreatedAt, u.CreatedBy = now, who
	u.UpdatedAt, u.UpdatedBy = now, who
	cp := *u
	r.byID[u.ID] = &cp
	return nil
}

// FindByID 按 ID 查找，未找到返回 (nil, nil)。
func (r *UserRepo) FindByID(_ context.Context, id string) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if u, ok := r.byID[id]; ok {
		cp := *u
		return &cp, nil
	}
	return nil, nil
}

// FindByUsername 按登录名查找，未找到返回 (nil, nil)。
func (r *UserRepo) FindByUsername(_ context.Context, username string) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, u := range r.byID {
		if u.Username == username {
			cp := *u
			return &cp, nil
		}
	}
	return nil, nil
}

// List 按创建时间稳定排序分页。
func (r *UserRepo) List(_ context.Context, q paging.PageRequest) (paging.Pageable[domain.User], error) {
	q = q.Normalize()
	r.mu.RLock()
	all := make([]domain.User, 0, len(r.byID))
	for _, u := range r.byID {
		all = append(all, *u)
	}
	r.mu.RUnlock()
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.Before(all[j].CreatedAt) })

	start, end := q.Offset(), q.Offset()+q.Limit()
	if start >= len(all) {
		return paging.NewPageable[domain.User](q, int64(len(all)), nil), nil
	}
	if end > len(all) {
		end = len(all)
	}
	return paging.NewPageable(q, int64(len(all)), all[start:end]), nil
}

// Delete 删除，目标不存在返回 nil（幂等，语义判断在 app 层）。
func (r *UserRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byID, id)
	return nil
}

var _ domain.UserRepository = (*UserRepo)(nil)
