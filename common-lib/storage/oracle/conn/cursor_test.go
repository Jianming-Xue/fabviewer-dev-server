package conn

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	"github.com/fab-viewer/common-lib/common/bizerror"
)

func TestCursorLifecycle(t *testing.T) {
	sqlDB, mock := newMockDB(t)
	mock.ExpectQuery("SELECT id, name FROM t").
		WillReturnRows(sqlmock.NewRows([]string{"ID", "NAME"}).
			AddRow(1, "a").AddRow(2, "b"))

	rows, err := sqlDB.QueryContext(context.Background(), "SELECT id, name FROM t")
	assert.NoError(t, err)
	cur := &Cursor{rows: rows}

	cols, err := cur.Columns()
	assert.NoError(t, err)
	assert.Equal(t, []string{"ID", "NAME"}, cols)

	got := make([][2]any, 0, 2)
	for cur.Next() {
		var id int
		var name string
		assert.NoError(t, cur.Scan(&id, &name))
		got = append(got, [2]any{id, name})
	}
	assert.Equal(t, [2]any{1, "a"}, got[0])
	assert.Equal(t, [2]any{2, "b"}, got[1])
	assert.NoError(t, cur.Err())
	assert.NoError(t, cur.Close())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCursorScanMismatch(t *testing.T) {
	sqlDB, mock := newMockDB(t)
	mock.ExpectQuery("SELECT n FROM t").WillReturnRows(
		sqlmock.NewRows([]string{"N"}).AddRow("not-an-int"))
	rows, err := sqlDB.QueryContext(context.Background(), "SELECT n FROM t")
	assert.NoError(t, err)
	cur := &Cursor{rows: rows}

	assert.True(t, cur.Next())
	var n int
	err = cur.Scan(&n)
	assert.Error(t, err)
	e, ok := bizerror.From(err)
	assert.True(t, ok)
	assert.Equal(t, bizerror.CodeDB, e.Code)
	assert.NoError(t, cur.Close())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCursorAfterClose(t *testing.T) {
	sqlDB, mock := newMockDB(t)
	mock.ExpectQuery("SELECT 1 FROM DUAL").WillReturnRows(
		sqlmock.NewRows([]string{"1"}).AddRow(1))
	rows, err := sqlDB.QueryContext(context.Background(), "SELECT 1 FROM DUAL")
	assert.NoError(t, err)
	cur := &Cursor{rows: rows}
	assert.NoError(t, cur.Close())

	// 关闭后：Next false，Scan/Columns 报错，Close 幂等
	assert.False(t, cur.Next())
	assert.Error(t, cur.Scan(new(int)))
	_, err = cur.Columns()
	assert.Error(t, err)
	assert.NoError(t, cur.Close())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCursorZeroValue(t *testing.T) {
	var cur *Cursor
	assert.False(t, cur.Next())
	assert.NoError(t, cur.Close())

	err := cur.Scan()
	assert.Error(t, err)
	_, err = cur.Columns()
	assert.Error(t, err)
	assert.Error(t, cur.Err())

	zero := &Cursor{}
	assert.False(t, zero.Next())
	assert.NoError(t, zero.Close())
	assert.Error(t, zero.Scan())
	_, err = zero.Columns()
	assert.Error(t, err)
	assert.Error(t, zero.Err())
}

func TestOutCursorInvalid(t *testing.T) {
	assert.NotPanics(t, func() {
		out := OutCursor(nil)
		m, ok := out.(*outCursor)
		assert.True(t, ok, "应返回游标标记类型")
		assert.Error(t, m.err)
	})
}
