package domain

import (
	"context"

	"github.com/fab-viewer/common-lib/common/paging"
)

// UserRepository 是用户聚合的存储端口（查询与写入）。实现方约定：
//
//   - Find* 未找到时返回 (nil, nil)——"有没有"是仓储语义，翻译成什么业务
//     错误（消息、编码、语言）由 app 层决定
//   - 其余失败返回技术错误（根因），app 层负责包装
//   - 写操作的审计字段（CreatedBy/UpdatedBy 等）由实现方从上下文操作者
//     填充（oracle 实现经 BasePO 钩子自动完成）
type UserRepository interface {
	Create(ctx context.Context, u *User) error
	FindByID(ctx context.Context, id string) (*User, error)
	FindByUsername(ctx context.Context, username string) (*User, error)
	List(ctx context.Context, q paging.Query) (paging.Pageable[User], error)
	Delete(ctx context.Context, id string) error
}

// TxManager 是事务端口：fn 在同一事务内执行，任一步出错整体回滚。
// 嵌套调用复用外层事务（实现方保证）。
type TxManager interface {
	WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}
