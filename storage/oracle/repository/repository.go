// Package repository 提供基于 base.BasePO 的通用审计仓储：Create 自动生成
// 雪花 ID，审计字段由 BasePO 钩子自动填充，业务仓储按类型参数复用。
package repository

import (
	"context"

	"github.com/fab-viewer/common-lib/common/errs"
	"github.com/fab-viewer/common-lib/storage/oracle/base"
	"github.com/fab-viewer/common-lib/storage/oracle/conn"
	"github.com/fab-viewer/common-lib/storage/snowflake"
)

// Repository 是基于 BasePO 的通用审计仓储：Create 自动生成雪花 ID
// （snowflake 包，多实例启动时须 snowflake.SetNode 各配不同节点号），
// 审计字段由 BasePO 钩子自动填充；Update 全量保存并刷新修改人/修改时间。
//
// 用法：
//
//	type orderPO struct {
//	    base.BasePO
//	    OrderNo string
//	}
//	repo, err := repository.NewRepository[orderPO](db)
//	err = repo.Create(ctx, &po)  // ctx 来自 middleware.RuntimeContext（userName）或 actor.NewContext
//	err = repo.Update(ctx, &po)
//
// T 必须嵌入 BasePO，否则 Create/Update 返回校验错误；需要之外的写法时
// 走 conn.DB 的 GORM() 出口。
type Repository[T any] struct {
	db *conn.DB
}

// NewRepository 构建仓储。
func NewRepository[T any](db *conn.DB) (*Repository[T], error) {
	if db == nil {
		return nil, errs.New(errs.CodeValidation, "repository.NewRepository db 不能为 nil")
	}
	return &Repository[T]{db: db}, nil
}

// Create 自动生成 ID（ID 为空时取雪花）并插入；审计字段由钩子填充。
func (r *Repository[T]) Create(ctx context.Context, po *T) error {
	bp, err := baseOf(po)
	if err != nil {
		return err
	}
	if bp.ID == "" {
		bp.ID = snowflake.Next()
	}
	if err := r.db.GORM().WithContext(ctx).Create(po).Error; err != nil {
		return errs.Wrap(err, errs.CodeDB, "repository.Create")
	}
	return nil
}

// Update 全量保存（Save）并经钩子刷新修改人/修改时间；主键必须已赋值。
func (r *Repository[T]) Update(ctx context.Context, po *T) error {
	if _, err := baseOf(po); err != nil {
		return err
	}
	if err := r.db.GORM().WithContext(ctx).Save(po).Error; err != nil {
		return errs.Wrap(err, errs.CodeDB, "repository.Update")
	}
	return nil
}

// baseOf 校验 T 嵌入了 BasePO 并返回其落址（未导出方法接口，外部类型无法伪造）。
func baseOf[T any](po *T) (*base.BasePO, error) {
	v, ok := any(po).(interface{ auditBase() *base.BasePO })
	if !ok || v == nil {
		return nil, errs.New(errs.CodeValidation,
			"repository.Repository 的类型参数必须嵌入 base.BasePO")
	}
	return v.auditBase(), nil
}
