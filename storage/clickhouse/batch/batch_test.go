package batch

import (
	"context"
	"errors"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"

	"github.com/fab-viewer/common-lib/common/errs"
	clickhouse "github.com/fab-viewer/common-lib/storage/clickhouse"
)

// chRow 是 batch/queryslice 测试共用的行结构，ch tag 与测试表列对应。
type chRow struct {
	ID   int64  `ch:"id"`
	Name string `ch:"name"`
}

// newTestBatch 建立挂在 mock 替身上的 Batch（size 默认 0，按用例覆盖）。
func newTestBatch(t *testing.T) (*Batch[chRow], *MockBatchSender) {
	t.Helper()
	ctrl := gomock.NewController(t)
	pb, sender := NewMockPrepareBatcher(ctrl), NewMockBatchSender(ctrl)
	pb.EXPECT().PrepareBatch(gomock.Any(), "INSERT INTO t").Return(sender, nil)
	b, err := newBatch[chRow](context.Background(), pb, "t", 0)
	require.NoError(t, err)
	require.NotNil(t, b)
	return b, sender
}

// asDBError 断言 err 是 errs.CodeDB 并返回它。
func asDBError(t *testing.T, err error) *errs.Error {
	t.Helper()
	var e *errs.Error
	require.ErrorAs(t, err, &e)
	assert.Equal(t, errs.CodeDB, e.Code)
	return e
}

func TestNewBatch(t *testing.T) {
	t.Run("成功并记录表名与阈值", func(t *testing.T) {
		b, sender := newTestBatch(t)
		assert.Equal(t, "t", b.table)
		assert.Equal(t, 0, b.size)
		assert.Equal(t, sender, b.sender)
		assert.False(t, b.closed)
	})

	t.Run("PrepareBatch 失败返回 CodeDB", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pb := NewMockPrepareBatcher(ctrl)
		pb.EXPECT().PrepareBatch(gomock.Any(), "INSERT INTO t").
			Return(nil, errors.New("prepare boom"))
		_, err := newBatch[chRow](context.Background(), pb, "t", 10)
		e := asDBError(t, err)
		assert.Contains(t, e.Message, "clickhouse.NewBatch table=t")
	})

	t.Run("反射/网络路径 panic 转错误", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pb := NewMockPrepareBatcher(ctrl)
		pb.EXPECT().PrepareBatch(gomock.Any(), gomock.Any()).
			Do(func(context.Context, string, ...driver.PrepareBatchOption) { panic("kaboom") })
		_, err := newBatch[chRow](context.Background(), pb, "t", 10)
		e := asDBError(t, err)
		assert.Contains(t, e.Message, "clickhouse.NewBatch table=t: kaboom")
	})

	t.Run("经 Client 建立通道并透传底层驱动", func(t *testing.T) {
		conn := &fakeConn{}
		b, err := NewBatch[chRow](context.Background(), clickhouse.NewFromConn(conn), "t", 10)
		require.NoError(t, err)
		require.NotNil(t, b)
		assert.Equal(t, []string{"INSERT INTO t"}, conn.prepareCall)
		assert.Equal(t, 10, b.size)
	})

	t.Run("底层驱动失败经适配器透传", func(t *testing.T) {
		c := clickhouse.NewFromConn(&fakeConn{prepareErr: errors.New("prepare boom")})
		_, err := NewBatch[chRow](context.Background(), c, "t", 10)
		e := asDBError(t, err)
		assert.Contains(t, e.Message, "clickhouse.NewBatch table=t")
	})
}

func TestBatchAppend(t *testing.T) {
	t.Run("达到阈值自动发送并重置计数", func(t *testing.T) {
		b, sender := newTestBatch(t)
		b.size = 2
		sender.EXPECT().AppendStruct(gomock.Any()).Times(4).Return(nil)
		sender.EXPECT().Flush().Times(2) // 第 2、4 行各触发一次

		for i := int64(1); i <= 4; i++ {
			require.NoError(t, b.Append(chRow{ID: i, Name: "n"}))
		}
		assert.Equal(t, 0, b.count)
	})

	t.Run("未达阈值不发送", func(t *testing.T) {
		b, sender := newTestBatch(t)
		b.size = 10
		sender.EXPECT().AppendStruct(gomock.Any()).Times(3).Return(nil)
		for i := 1; i <= 3; i++ {
			require.NoError(t, b.Append(chRow{ID: int64(i)}))
		}
		assert.Equal(t, 3, b.count)
	})

	t.Run("阈值为 0 表示不自动发送", func(t *testing.T) {
		b, sender := newTestBatch(t) // 无 Flush EXPECT：若被自动调用即测试失败
		sender.EXPECT().AppendStruct(gomock.Any()).Times(5).Return(nil)
		for i := 1; i <= 5; i++ {
			require.NoError(t, b.Append(chRow{ID: int64(i)}))
		}
		assert.Equal(t, 5, b.count)
	})

	t.Run("按 ch tag 反射追加结构体指针", func(t *testing.T) {
		b, sender := newTestBatch(t)
		b.size = 100
		sender.EXPECT().AppendStruct(gomock.Any()).
			DoAndReturn(func(v any) error {
				r, ok := v.(*chRow)
				require.True(t, ok)
				assert.Equal(t, int64(7), r.ID)
				assert.Equal(t, "seven", r.Name)
				return nil
			})
		require.NoError(t, b.Append(chRow{ID: 7, Name: "seven"}))
	})

	t.Run("追加失败错误传播", func(t *testing.T) {
		b, sender := newTestBatch(t)
		b.size = 1
		sender.EXPECT().AppendStruct(gomock.Any()).Return(errors.New("append boom"))
		err := b.Append(chRow{ID: 1})
		e := asDBError(t, err)
		assert.Contains(t, e.Message, "clickhouse.Batch.Append table=t")
		assert.Equal(t, 0, b.count) // 失败不计数
	})

	t.Run("反射路径 panic 转错误", func(t *testing.T) {
		b, sender := newTestBatch(t)
		sender.EXPECT().AppendStruct(gomock.Any()).Do(func(any) { panic("boom") })
		err := b.Append(chRow{ID: 1})
		e := asDBError(t, err)
		assert.Contains(t, e.Message, "clickhouse.Batch.Append table=t: boom")
	})

	t.Run("自动发送失败包装上下文且计数保留待重试", func(t *testing.T) {
		b, sender := newTestBatch(t)
		b.size = 1
		gomock.InOrder(
			sender.EXPECT().AppendStruct(gomock.Any()).Return(nil),
			sender.EXPECT().Flush().Return(errors.New("flush boom")),
			sender.EXPECT().AppendStruct(gomock.Any()).Return(nil),
			sender.EXPECT().Flush().Return(nil), // 下一次 Append 重试发送成功
		)
		err := b.Append(chRow{ID: 1})
		e := asDBError(t, err)
		assert.Contains(t, e.Message, "clickhouse.Batch.Append table=t size=1 达到阈值自动发送失败")
		assert.Equal(t, 1, b.count) // 计数未重置

		require.NoError(t, b.Append(chRow{ID: 2}))
		assert.Equal(t, 0, b.count)
	})

	t.Run("关闭后追加报错且不触达底层", func(t *testing.T) {
		b, _ := newTestBatch(t) // 无任何 EXPECT：底层被调用即测试失败
		b.closed = true
		err := b.Append(chRow{ID: 1})
		e := asDBError(t, err)
		assert.Contains(t, e.Message, "clickhouse.Batch.Append table=t: batch 已关闭")
	})
}

func TestBatchFlush(t *testing.T) {
	t.Run("显式发送并重置计数", func(t *testing.T) {
		b, sender := newTestBatch(t)
		b.size = 10
		sender.EXPECT().AppendStruct(gomock.Any()).Times(2).Return(nil)
		sender.EXPECT().Flush().Return(nil)
		require.NoError(t, b.Append(chRow{ID: 1}))
		require.NoError(t, b.Append(chRow{ID: 2}))
		assert.Equal(t, 2, b.count)

		require.NoError(t, b.Flush(context.Background()))
		assert.Equal(t, 0, b.count)
	})

	t.Run("ctx 取消不发起发送", func(t *testing.T) {
		b, _ := newTestBatch(t) // 无 Flush EXPECT：被调用即失败
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := b.Flush(ctx)
		e := asDBError(t, err)
		assert.Contains(t, e.Message, "clickhouse.Batch.Flush table=t: ctx 已取消")
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("发送失败错误传播", func(t *testing.T) {
		b, sender := newTestBatch(t)
		sender.EXPECT().Flush().Return(errors.New("flush boom"))
		err := b.Flush(context.Background())
		e := asDBError(t, err)
		assert.Contains(t, e.Message, "clickhouse.Batch.Flush table=t")
	})

	t.Run("关闭后 Flush 报错", func(t *testing.T) {
		b, _ := newTestBatch(t)
		b.closed = true
		err := b.Flush(context.Background())
		e := asDBError(t, err)
		assert.Contains(t, e.Message, "clickhouse.Batch.Flush table=t: batch 已关闭")
	})
}

func TestBatchClose(t *testing.T) {
	t.Run("发送剩余行并收尾且幂等", func(t *testing.T) {
		b, sender := newTestBatch(t)
		b.size = 100
		sender.EXPECT().AppendStruct(gomock.Any()).Times(3).Return(nil)
		for i := 1; i <= 3; i++ {
			require.NoError(t, b.Append(chRow{ID: int64(i)}))
		}
		sender.EXPECT().Send().Return(nil)
		sender.EXPECT().Close().Return(nil)

		require.NoError(t, b.Close(context.Background()))
		assert.True(t, b.closed)
		require.NoError(t, b.Close(context.Background())) // 幂等：不再触达底层
	})

	t.Run("Send 失败仍收尾并传播错误", func(t *testing.T) {
		b, sender := newTestBatch(t)
		sender.EXPECT().Send().Return(errors.New("send boom"))
		sender.EXPECT().Close().Return(nil)
		err := b.Close(context.Background())
		e := asDBError(t, err)
		assert.Contains(t, e.Message, "clickhouse.Batch.Close table=t Send")
		assert.True(t, b.closed)
	})

	t.Run("底层 Close 失败传播错误", func(t *testing.T) {
		b, sender := newTestBatch(t)
		sender.EXPECT().Send().Return(nil)
		sender.EXPECT().Close().Return(errors.New("close boom"))
		err := b.Close(context.Background())
		e := asDBError(t, err)
		assert.Contains(t, e.Message, "clickhouse.Batch.Close table=t Close")
	})

	t.Run("ctx 取消时不发送仅释放连接", func(t *testing.T) {
		b, sender := newTestBatch(t)
		sender.EXPECT().Close().Return(nil) // 释放连接；无 Send EXPECT：不得发送
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := b.Close(ctx)
		e := asDBError(t, err)
		assert.Contains(t, e.Message, "clickhouse.Batch.Close table=t: ctx 已取消")
		assert.ErrorIs(t, err, context.Canceled)
		assert.True(t, b.closed)
		require.NoError(t, b.Close(ctx)) // 幂等
	})
}
