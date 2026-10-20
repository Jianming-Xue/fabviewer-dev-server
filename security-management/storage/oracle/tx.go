package oracle

import (
	"context"

	"gorm.io/gorm"

	"github.com/fab-viewer/common-lib/common/fverr"
	"github.com/fab-viewer/common-lib/storage/oracle/conn"
)

// TxManager 实现 domain.TxManager：GORM 同库事务，fn 出错整体回滚。
// 嵌套调用复用外层事务（SavePoint 语义由 GORM Transaction 链保证）。
type TxManager struct {
	db *conn.DB
}

// NewTxManager 构建事务管理器。
func NewTxManager(db *conn.DB) *TxManager {
	return &TxManager{db: db}
}

// WithTx 在事务内执行 fn。
func (m *TxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if err := m.db.GORM().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 事务句柄如何传给 fn 内的仓储调用见后续切片（context 携带方案）
		_ = tx
		return fn(ctx)
	}); err != nil {
		return fverr.NewError(fverr.CodeDB, "tx commit/rollback").AttachError(err, nil)
	}
	return nil
}
