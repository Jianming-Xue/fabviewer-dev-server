package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRows 是手写的 driver.Rows 替身：由闭包驱动行为，模拟 SDK 的
// Next/ScanStruct（ch tag 填充）/Err/Close 语义。
type fakeRows struct {
	driver.Rows // 嵌入接口：未覆盖方法被调用即 panic，由包边界 recover 兜底并暴露
	next        func() bool
	scanStruct  func(dest any) error
	err         error
	closeErr    error
	closed      bool
}

func (r *fakeRows) Next() bool                { return r.next() }
func (r *fakeRows) ScanStruct(dest any) error { return r.scanStruct(dest) }
func (r *fakeRows) Err() error                { return r.err }
func (r *fakeRows) Close() error              { r.closed = true; return r.closeErr }

// rowsFrom 构造按序返回 rows 的 fakeRows。
func rowsFrom(rows ...chRow) *fakeRows {
	i := 0
	return &fakeRows{
		next: func() bool {
			i++
			return i <= len(rows)
		},
		scanStruct: func(dest any) error {
			return fillRow(dest, rows[i-1].ID, rows[i-1].Name)
		},
	}
}

// fillRow 模拟 SDK ScanStruct 的 ch tag 反射填充。
func fillRow(dest any, id int64, name string) error {
	r, ok := dest.(*chRow)
	if !ok {
		return fmt.Errorf("clickhouse [ScanStruct]: (%T) expects a struct dest", dest)
	}
	r.ID, r.Name = id, name
	return nil
}

func TestQuerySlice(t *testing.T) {
	t.Run("按 ch tag 填充多行并关闭结果集", func(t *testing.T) {
		fr := rowsFrom(chRow{ID: 1, Name: "a"}, chRow{ID: 2, Name: "b"}, chRow{ID: 3, Name: "c"})
		c := &Client{conn: &fakeConn{queryRows: fr}}

		got, err := QuerySlice[chRow](context.Background(), c, "SELECT id, name FROM t")
		require.NoError(t, err)
		assert.Equal(t, []chRow{{ID: 1, Name: "a"}, {ID: 2, Name: "b"}, {ID: 3, Name: "c"}}, got)
		assert.True(t, fr.closed)
		assert.GreaterOrEqual(t, cap(got), querySliceInitialCap) // 未知行数给初始容量
	})

	t.Run("空结果返回非 nil 空切片", func(t *testing.T) {
		fr := rowsFrom()
		c := &Client{conn: &fakeConn{queryRows: fr}}

		got, err := QuerySlice[chRow](context.Background(), c, "SELECT id FROM t WHERE 1=0")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Empty(t, got)
		assert.True(t, fr.closed)
	})

	t.Run("查询失败带上下文", func(t *testing.T) {
		c := &Client{conn: &fakeConn{queryErr: errors.New("query boom")}}
		_, err := QuerySlice[chRow](context.Background(), c, "SELECT boom")
		e := asDBError(t, err)
		assert.Contains(t, e.Message, "clickhouse.QuerySlice query=SELECT boom")
	})

	t.Run("列缺失或类型不匹配报错并定位行号", func(t *testing.T) {
		fr := rowsFrom(chRow{ID: 1})
		fr.scanStruct = func(any) error {
			return errors.New(`missing destination name "extra" in *clickhouse.chRow`)
		}
		c := &Client{conn: &fakeConn{queryRows: fr}}

		_, err := QuerySlice[chRow](context.Background(), c, "SELECT id, extra FROM t")
		e := asDBError(t, err)
		assert.Contains(t, e.Message, "clickhouse.QuerySlice query=SELECT id, extra FROM t 第 1 行填充失败")
		assert.True(t, fr.closed)
	})

	t.Run("遍历后的 rows 错误传播", func(t *testing.T) {
		fr := rowsFrom(chRow{ID: 1, Name: "a"})
		fr.err = errors.New("rows boom")
		c := &Client{conn: &fakeConn{queryRows: fr}}

		_, err := QuerySlice[chRow](context.Background(), c, "SELECT id FROM t")
		e := asDBError(t, err)
		assert.Contains(t, e.Message, "clickhouse.QuerySlice query=SELECT id FROM t")
	})

	t.Run("非结构体类型参数直接报错", func(t *testing.T) {
		conn := &fakeConn{}
		c := &Client{conn: conn}

		_, err := QuerySlice[int](context.Background(), c, "SELECT 1")
		e := asDBError(t, err)
		assert.Contains(t, e.Message, "clickhouse.QuerySlice query=SELECT 1: 类型参数 int 非结构体")
		assert.Empty(t, conn.queryCalls) // 校验前置，未发起查询
	})

	t.Run("反射路径 panic 转错误", func(t *testing.T) {
		fr := rowsFrom(chRow{ID: 1})
		fr.scanStruct = func(any) error { panic("scan kaboom") }
		c := &Client{conn: &fakeConn{queryRows: fr}}

		_, err := QuerySlice[chRow](context.Background(), c, "SELECT id FROM t")
		e := asDBError(t, err)
		assert.Contains(t, e.Message, "clickhouse.QuerySlice query=SELECT id FROM t: scan kaboom")
		assert.True(t, fr.closed) // recover 前已由 defer 关闭
	})
}
