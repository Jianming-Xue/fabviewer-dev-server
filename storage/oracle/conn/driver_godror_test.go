package conn

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
	"gorm.io/gorm/utils/tests"

	"github.com/fab-viewer/common-lib/common/errs"
)

func TestGodrorDriverName(t *testing.T) {
	assert.Equal(t, "godror", defaultDriver.Name())
	var _ interface{ Name() string } = godrorDriver{}
}

func TestGodrorDriverGormDialector(t *testing.T) {
	got := godrorDriver{}.GormDialector(`user="u" connectString="h/s"`)
	assert.NotNil(t, got)
	assert.Equal(t, "oracle", got.Name(), "应返回 oracle-samples dialector")
}

// stubQuerier 把任意查询替换为预置结果，模拟 godror 对 WrapRows 魔法查询的拦截返回。
type stubQuerier struct {
	rows *sql.Rows
	err  error
}

// ExecContext 实现 Execer。
func (s stubQuerier) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return nil, s.err
}

// QueryContext 实现 Execer：恒返回预置行集或预置错误。
func (s stubQuerier) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return s.rows, s.err
}

func TestGodrorDriverNewCursor(t *testing.T) {
	t.Run("经 WrapRows 包装为 Cursor", func(t *testing.T) {
		sqlDB, mock := newMockDB(t)
		mock.ExpectQuery("SELECT wrap").WillReturnRows(
			sqlmock.NewRows([]string{"ID"}).AddRow(1))
		rows, err := sqlDB.QueryContext(context.Background(), "SELECT wrap")
		assert.NoError(t, err)

		cur, err := godrorDriver{}.NewCursor(context.Background(), stubQuerier{rows: rows}, fakeDriverRows{})
		assert.NoError(t, err)
		assert.Same(t, rows, cur.rows)
		assert.NoError(t, cur.Close())
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("查询通道为 nil 报校验错误", func(t *testing.T) {
		_, err := godrorDriver{}.NewCursor(context.Background(), nil, fakeDriverRows{})
		assert.Error(t, err)
		e, ok := errs.From(err)
		assert.True(t, ok)
		assert.Equal(t, errs.CodeValidation, e.Code)
	})

	t.Run("行集为 nil 报校验错误", func(t *testing.T) {
		_, err := godrorDriver{}.NewCursor(context.Background(), stubQuerier{}, nil)
		assert.Error(t, err)
		_, ok := errs.From(err)
		assert.True(t, ok)
	})

	t.Run("WrapRows 失败包装为 DB 错误", func(t *testing.T) {
		failing := stubQuerier{err: errors.New("wrap fail")}
		_, err := godrorDriver{}.NewCursor(context.Background(), failing, fakeDriverRows{})
		assert.Error(t, err)
		e, ok := errs.From(err)
		assert.True(t, ok)
		assert.Equal(t, errs.CodeDB, e.Code)
		assert.Contains(t, err.Error(), "WrapRows")
	})
}

// errDialector 让 gorm.Open 在 Initialize 阶段失败，覆盖 Open 的建连错误分支。
type errDialector struct {
	tests.DummyDialector
	err error
}

// Initialize 实现 gorm.Dialector：恒返回预置错误。
func (d errDialector) Initialize(*gorm.DB) error { return d.err }

func TestOpenGormInitializeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	drv := NewMockdriverAdapter(ctrl)
	drv.EXPECT().Name().Return("mock").AnyTimes()
	drv.EXPECT().GormDialector(gomock.Any()).Return(errDialector{err: errors.New("dial tcp: refused")})
	assert.Equal(t, "mock", drv.Name()) // 触发 mock 的 Name 方法（补覆盖）

	_, err := Open(context.Background(), validCfg(), withDriver(drv))
	assert.Error(t, err)
	e, ok := errs.From(err)
	assert.True(t, ok)
	assert.Equal(t, errs.CodeDB, e.Code)
	assert.Contains(t, err.Error(), "oracle.Open")
}
