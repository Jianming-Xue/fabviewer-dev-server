// Package oracle 提供 domain 端口的 Oracle 实现：PO 映射经 GORM，
// 审计字段与雪花 ID 由 common-lib 的 BasePO 钩子/仓储自动处理，
// 事务经 TxManager（同库 GORM 事务）。
package oracle

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/fab-viewer/common-lib/common/paging"
	"github.com/fab-viewer/common-lib/storage/oracle/base"
	"github.com/fab-viewer/common-lib/storage/oracle/conn"
	"github.com/fab-viewer/common-lib/storage/oracle/repository"
	"github.com/fab-viewer/security-management/domain"
)

// userPO 是 SM_USER 表的持久化对象：嵌入 BasePO 获得 ID 与审计字段
// （列名全大写对齐 Oracle DDL，见 schema.sql）。
type userPO struct {
	base.BasePO
	Username string `gorm:"column:USERNAME;size:64;uniqueIndex"`
	Name     string `gorm:"column:NAME;size:128"`
	Email    string `gorm:"column:EMAIL;size:128"`
	Enabled  int    `gorm:"column:ENABLED"`
}

// TableName 表名。
func (userPO) TableName() string { return "SM_USER" }

// UserRepo 实现 domain.UserRepository。
type UserRepo struct {
	write *repository.Repository[userPO] // Create/Update 走通用仓储（审计钩子）
	db    *gorm.DB                       // 查询走 GORM 出口
}

// NewUserRepo 构建 Oracle 用户仓储。
func NewUserRepo(db *conn.DB) (*UserRepo, error) {
	write, err := repository.NewRepository[userPO](db)
	if err != nil {
		return nil, err
	}
	return &UserRepo{write: write, db: db.GORM()}, nil
}

// Create 插入（雪花 ID 与审计字段由通用仓储自动填充）。
func (r *UserRepo) Create(ctx context.Context, u *domain.User) error {
	po := toPO(u)
	if err := r.write.Create(ctx, &po); err != nil {
		return err
	}
	u.ID = po.ID
	u.CreatedAt, u.UpdatedAt = po.CreatedAt, po.UpdatedAt
	u.CreatedBy, u.UpdatedBy = po.CreatedBy, po.UpdatedBy
	return nil
}

// FindByID 查找，未找到返回 (nil, nil)。
func (r *UserRepo) FindByID(ctx context.Context, id string) (*domain.User, error) {
	var po userPO
	err := r.db.WithContext(ctx).Where("ID = ?", id).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toDomain(&po), nil
}

// FindByUsername 查找，未找到返回 (nil, nil)。
func (r *UserRepo) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	var po userPO
	err := r.db.WithContext(ctx).Where("USERNAME = ?", username).First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toDomain(&po), nil
}

// List 分页查询（按创建时间倒序）。
func (r *UserRepo) List(ctx context.Context, q paging.Query) (paging.Pageable[domain.User], error) {
	q = q.Normalize()
	var total int64
	if err := r.db.WithContext(ctx).Model(&userPO{}).Count(&total).Error; err != nil {
		return paging.Pageable[domain.User]{}, err
	}
	if total == 0 {
		return paging.NewPageable[domain.User](q, 0, nil), nil
	}
	var pos []userPO
	if err := r.db.WithContext(ctx).
		Order("CREATED_AT DESC").
		Offset(q.Offset()).Limit(q.Limit()).
		Find(&pos).Error; err != nil {
		return paging.Pageable[domain.User]{}, err
	}
	users := make([]domain.User, 0, len(pos))
	for i := range pos {
		users = append(users, *toDomain(&pos[i]))
	}
	return paging.NewPageable(q, total, users), nil
}

// Delete 删除。
func (r *UserRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("ID = ?", id).Delete(&userPO{}).Error
}

// toDomain PO 转实体。
func toDomain(po *userPO) *domain.User {
	return &domain.User{
		ID:        po.ID,
		Username:  po.Username,
		Name:      po.Name,
		Email:     po.Email,
		Enabled:   po.Enabled == 1,
		CreatedAt: po.CreatedAt,
		UpdatedAt: po.UpdatedAt,
		CreatedBy: po.CreatedBy,
		UpdatedBy: po.UpdatedBy,
	}
}

// toPO 实体转 PO（Enabled 布尔转 NUMBER(1)）。
func toPO(u *domain.User) userPO {
	enabled := 0
	if u.Enabled {
		enabled = 1
	}
	return userPO{
		BasePO:   base.BasePO{ID: u.ID},
		Username: u.Username,
		Name:     u.Name,
		Email:    u.Email,
		Enabled:  enabled,
	}
}

var _ domain.UserRepository = (*UserRepo)(nil)
