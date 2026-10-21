package base

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/fab-viewer/common-lib/common/rctx"
)

// newHookDB 构造带 ctx 的最小 *gorm.DB（钩子只读 Statement.Context，
// 手工装配 Config/Statement 避免拉起真实连接）。
func newHookDB(ctx context.Context) *gorm.DB {
	return &gorm.DB{
		Config:    &gorm.Config{},
		Statement: &gorm.Statement{Context: ctx},
	}
}

func TestBeforeCreateFillsAuditFields(t *testing.T) {
	ctx := rctx.SetRuntimeContext(context.Background(), rctx.RuntimeContext{}.WithUser("zhangsan"))
	po := BasePO{ID: "1.2.3"}

	require.NoError(t, po.BeforeCreate(newHookDB(ctx)))

	assert.Equal(t, "zhangsan", po.CreatedBy)
	assert.Equal(t, "zhangsan", po.UpdatedBy)
	assert.False(t, po.CreatedAt.IsZero())
	assert.False(t, po.UpdatedAt.IsZero())
}

func TestBeforeCreateKeepsManualValues(t *testing.T) {
	ctx := rctx.SetRuntimeContext(context.Background(), rctx.RuntimeContext{}.WithUser("zhangsan"))
	manual := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	po := BasePO{ID: "1", CreatedBy: "manual", CreatedAt: manual}

	require.NoError(t, po.BeforeCreate(newHookDB(ctx)))

	assert.Equal(t, "manual", po.CreatedBy, "手工赋值不覆盖")
	assert.Equal(t, manual, po.CreatedAt)
	assert.Equal(t, "zhangsan", po.UpdatedBy, "未手工赋值的字段仍填充")
}

func TestBeforeCreateWithoutActor(t *testing.T) {
	po := BasePO{ID: "1"}

	require.NoError(t, po.BeforeCreate(newHookDB(context.Background())))

	assert.Empty(t, po.CreatedBy, "无操作者时留空")
	assert.False(t, po.CreatedAt.IsZero(), "时间仍填充")
}

func TestBeforeUpdateRefreshes(t *testing.T) {
	ctx := rctx.SetRuntimeContext(context.Background(), rctx.RuntimeContext{}.WithUser("lisi"))
	po := BasePO{ID: "1", CreatedBy: "zhangsan", UpdatedBy: "zhangsan"}

	require.NoError(t, po.BeforeUpdate(newHookDB(ctx)))

	assert.Equal(t, "zhangsan", po.CreatedBy, "创建人不被修改")
	assert.Equal(t, "lisi", po.UpdatedBy, "修改人刷新为当前操作者")
	assert.False(t, po.UpdatedAt.IsZero())
}

// TestAuditBaseSealed 验证嵌入 BasePO 的业务类型满足未导出方法接口
// （repository.baseOf 由此定位审计字段落址，外部类型无法伪造）。
func TestAuditBaseSealed(t *testing.T) {
	type orderPO struct {
		BasePO
		OrderNo string
	}

	po := orderPO{OrderNo: "A-1"}
	v, ok := any(&po).(interface{ auditBase() *BasePO })
	require.True(t, ok, "嵌入 BasePO 即满足未导出方法接口")
	assert.Equal(t, &po.BasePO, v.auditBase())

	var bare struct{ N int }
	_, ok = any(&bare).(interface{ auditBase() *BasePO })
	assert.False(t, ok, "未嵌入的类型不满足")
}
