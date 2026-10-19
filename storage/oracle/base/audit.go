package base

import (
	"time"

	"gorm.io/gorm"

	"github.com/fab-viewer/common-lib/common/actor"
)

// BasePO 是库提供的基础审计 PO：ID + 创建人/创建时间/修改人/修改时间五个
// 基础字段。业务 PO 嵌入它即获得审计能力（列为全大写命名，配合库内置的
// upperNamer 与标准 Oracle DDL 对齐）：
//
//	type orderPO struct {
//	    base.BasePO // ID/CREATED_BY/CREATED_AT/UPDATED_BY/UPDATED_AT
//	    OrderNo string
//	}
//
// 审计字段由 GORM 钩子自动填充（直接用 GORM 出口也生效）：
//   - BeforeCreate：创建/修改时间置当前时间，创建/修改人取上下文操作者
//     （api 层经 api.WithActor 注入，定时任务经 actor.NewContext 注入）；
//     已手工赋值的字段不覆盖
//   - BeforeUpdate：修改时间置当前时间、修改人取上下文操作者
//
// ID 是点分隔雪花 ID（snowflake 包，"<时间戳>.<节点>.<序列>"，列
// VARCHAR2(32)），由 Repository.Create 自动生成，或业务方自行赋值。
//
//nolint:revive // 命名沿用领域词汇 BasePO，分包后前缀重复可接受
type BasePO struct {
	ID        string    `gorm:"column:ID;primaryKey;size:32"`
	CreatedBy string    `gorm:"column:CREATED_BY;size:64"`
	CreatedAt time.Time `gorm:"column:CREATED_AT"`
	UpdatedBy string    `gorm:"column:UPDATED_BY;size:64"`
	UpdatedAt time.Time `gorm:"column:UPDATED_AT"`
}

// auditBase 是包内标记方法：只有嵌入了 BasePO 的类型才满足该接口，
// Repository 由此定位审计字段落址（外部类型无法伪造）。
func (b *BasePO) auditBase() *BasePO { return b }

// BeforeCreate 实现 GORM 钩子：填充创建/修改时间与创建/修改人。
// 空字段不覆盖手工赋值；操作者取 tx.Statement.Context（经
// GORM().WithContext(ctx) 传播）。
func (b *BasePO) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	if b.CreatedAt.IsZero() {
		b.CreatedAt = now
	}
	if b.UpdatedAt.IsZero() {
		b.UpdatedAt = now
	}
	who := actor.FromContext(tx.Statement.Context)
	if who != "" {
		if b.CreatedBy == "" {
			b.CreatedBy = who
		}
		if b.UpdatedBy == "" {
			b.UpdatedBy = who
		}
	}
	return nil
}

// BeforeUpdate 实现 GORM 钩子：刷新修改时间与修改人。
// 需要跳过审计时使用 UpdateColumns（GORM 约定不触发钩子）。
func (b *BasePO) BeforeUpdate(tx *gorm.DB) error {
	b.UpdatedAt = time.Now()
	if who := actor.FromContext(tx.Statement.Context); who != "" {
		b.UpdatedBy = who
	}
	return nil
}
