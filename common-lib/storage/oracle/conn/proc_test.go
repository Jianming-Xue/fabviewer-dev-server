package conn

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/fab-viewer/common-lib/common/fverr"
)

// procDB 构造直接挂 sqlmock 连接的 DB（跳过 Open，聚焦过程调用逻辑）。
func procDB(t *testing.T) (*DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock := newMockDB(t)
	return &DB{drv: defaultDriver, sqlDB: sqlDB}, mock
}

func TestCallProcSQLAndBindings(t *testing.T) {
	t.Run("无参数", func(t *testing.T) {
		db, mock := procDB(t)
		mock.ExpectExec("BEGIN P_NOARG; END;").WillReturnResult(sqlmock.NewResult(0, 0))
		assert.NoError(t, db.CallProc(context.Background(), "P_NOARG"))
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("纯 IN 参数按位置绑定", func(t *testing.T) {
		db, mock := procDB(t)
		mock.ExpectExec("BEGIN P_IN(:1, :2); END;").
			WithArgs("x", int64(42)).
			WillReturnResult(sqlmock.NewResult(0, 0))
		assert.NoError(t, db.CallProc(context.Background(), "P_IN", "x", 42))
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("标量 OUT 翻译为 sql.Out 且 Dest 指针同一", func(t *testing.T) {
		db, mock := procDB(t)
		var total int64
		mock.ExpectExec("BEGIN P_OUT(:1, :2); END;").
			WithArgs("IN1", outDestMatcher{desc: "*int64", dest: &total}).
			WillReturnResult(sqlmock.NewResult(0, 0))
		assert.NoError(t, db.CallProc(context.Background(), "P_OUT", "IN1", Out(&total)))
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestCallProcCursorBinding(t *testing.T) {
	t.Run("游标 OUT 占位绑定且未写回时安全跳过", func(t *testing.T) {
		db, mock := procDB(t)
		var cur Cursor
		mock.ExpectExec("BEGIN P_LIST(:1); END;").
			WithArgs(cursorDestMatcher{}).
			WillReturnResult(sqlmock.NewResult(0, 0))
		assert.NoError(t, db.CallProc(context.Background(), "P_LIST", OutCursor(&cur)))
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("驱动写回后经 driverAdapter 回填 Cursor", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		drv := NewMockdriverAdapter(ctrl)
		db, mock := procDB(t)
		db.drv = drv

		rowsSQL, rowsMock := newMockDB(t)
		rowsMock.ExpectQuery("SELECT refcursor").WillReturnRows(
			sqlmock.NewRows([]string{"ID"}).AddRow(1))
		wantRows, err := rowsSQL.QueryContext(context.Background(), "SELECT refcursor")
		assert.NoError(t, err)

		var cur Cursor
		arg := &outCursor{dest: &cur, rows: fakeDriverRows{}}
		mock.ExpectExec("BEGIN P_LIST(:1); END;").
			WithArgs(cursorDestMatcher{}).
			WillReturnResult(sqlmock.NewResult(0, 0))
		drv.EXPECT().NewCursor(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&Cursor{rows: wantRows}, nil)

		assert.NoError(t, db.CallProc(context.Background(), "P_LIST", arg))
		assert.Same(t, wantRows, cur.rows, "OutCursor 目标应被填充")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("NewCursor 失败时返回带过程名的包装错误", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		drv := NewMockdriverAdapter(ctrl)
		db, mock := procDB(t)
		db.drv = drv

		var cur Cursor
		arg := &outCursor{dest: &cur, rows: fakeDriverRows{}}
		mock.ExpectExec("BEGIN P_LIST(:1); END;").
			WithArgs(cursorDestMatcher{}).
			WillReturnResult(sqlmock.NewResult(0, 0))
		drv.EXPECT().NewCursor(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, errors.New("wrap failed"))

		err := db.CallProc(context.Background(), "P_LIST", arg)
		assert.Error(t, err)
		e, ok := fverr.From(err)
		assert.True(t, ok)
		assert.Equal(t, fverr.CodeDB, e.Code)
		assert.Contains(t, err.Error(), "P_LIST")
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestCallFuncSQLAndBindings(t *testing.T) {
	t.Run("无参函数生成 :1 := F", func(t *testing.T) {
		db, mock := procDB(t)
		var v int64
		mock.ExpectExec("BEGIN :1 := F_NOW; END;").
			WithArgs(outDestMatcher{desc: "*int64", dest: &v}).
			WillReturnResult(sqlmock.NewResult(0, 0))
		assert.NoError(t, db.CallFunc(context.Background(), "F_NOW", &v))
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("带参函数占位符从 :2 顺延", func(t *testing.T) {
		db, mock := procDB(t)
		var v string
		mock.ExpectExec("BEGIN :1 := F_FMT(:2, :3); END;").
			WithArgs(
				outDestMatcher{desc: "*string", dest: &v},
				"a",
				int64(1),
			).
			WillReturnResult(sqlmock.NewResult(0, 0))
		assert.NoError(t, db.CallFunc(context.Background(), "F_FMT", &v, "a", 1))
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("dest 非指针或 nil 返回校验错误", func(t *testing.T) {
		db, _ := procDB(t)
		err := db.CallFunc(context.Background(), "F", 42)
		assert.Error(t, err)
		e, ok := fverr.From(err)
		assert.True(t, ok)
		assert.Equal(t, fverr.CodeValidation, e.Code)
		assert.Error(t, db.CallFunc(context.Background(), "F", nil))
	})
}

func TestOutInvalidInput(t *testing.T) {
	db, mock := procDB(t)
	tests := []struct {
		name string
		arg  any
	}{
		{name: "Out(nil)", arg: Out(nil)},
		{name: "Out(非指针)", arg: Out(42)},
		{name: "Out(字符串)", arg: Out("s")},
		{name: "OutCursor(nil)", arg: OutCursor(nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				err := db.CallProc(context.Background(), "P", tt.arg)
				assert.Error(t, err)
				e, ok := fverr.From(err)
				assert.True(t, ok)
				assert.Equal(t, fverr.CodeValidation, e.Code)
			})
		})
	}
	assert.NoError(t, mock.ExpectationsWereMet(), "非法入参不应触达数据库")
}

func TestCallProcExecError(t *testing.T) {
	db, mock := procDB(t)
	mock.ExpectExec("BEGIN P_BOOM(:1); END;").
		WithArgs("x").
		WillReturnError(errors.New("ORA-01013"))

	err := db.CallProc(context.Background(), "P_BOOM", "x")
	assert.Error(t, err)
	e, ok := fverr.From(err)
	assert.True(t, ok)
	assert.Equal(t, fverr.CodeDB, e.Code)
	assert.Contains(t, err.Error(), "P_BOOM")
}

func TestCallProcRecoversPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		err := callProc(newCallCtx(context.Background(), defaultDriver, panicExecer{}, "P_PANIC"), []any{"x"})
		assert.Error(t, err)
		e, ok := fverr.From(err)
		assert.True(t, ok)
		assert.Equal(t, fverr.CodeValidation, e.Code)
		assert.Contains(t, err.Error(), "panic")
	})
}

func TestCallProcTxNil(t *testing.T) {
	assert.Error(t, CallProcTx(context.Background(), nil, "P"))
	assert.Error(t, CallFuncTx(context.Background(), nil, "F", new(int)))
}

func TestCallTxWithSQLTx(t *testing.T) {
	t.Run("CallProcTx 在 *sql.Tx 上执行", func(t *testing.T) {
		db, mock := procDB(t)
		ctx := context.Background()
		mock.ExpectBegin()
		mock.ExpectExec("BEGIN P_TX(:1); END;").
			WithArgs("v").
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectCommit()

		tx, err := db.sqlDB.BeginTx(ctx, nil)
		assert.NoError(t, err)
		assert.NoError(t, CallProcTx(ctx, tx, "P_TX", "v"))
		assert.NoError(t, tx.Commit())
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("CallFuncTx 在 *sql.Tx 上执行", func(t *testing.T) {
		db, mock := procDB(t)
		ctx := context.Background()
		var total int64
		mock.ExpectBegin()
		mock.ExpectExec("BEGIN :1 := F_SUM(:2, :3); END;").
			WithArgs(
				outDestMatcher{desc: "*int64", dest: &total},
				int64(1),
				int64(2),
			).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectCommit()

		tx, err := db.sqlDB.BeginTx(ctx, nil)
		assert.NoError(t, err)
		assert.NoError(t, CallFuncTx(ctx, tx, "F_SUM", &total, 1, 2))
		assert.NoError(t, tx.Commit())
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestFromGorm(t *testing.T) {
	t.Run("gorm.DB 连接池可提取为 Execer 并执行过程", func(t *testing.T) {
		sqlDB, mock := newMockDB(t)
		gdb, err := gorm.Open(connDialector{conn: sqlDB}, &gorm.Config{DisableAutomaticPing: true})
		assert.NoError(t, err)
		ex, err := FromGorm(gdb)
		assert.NoError(t, err)
		assert.NotNil(t, ex)

		mock.ExpectExec("BEGIN P_G(:1); END;").
			WithArgs(int64(1)).
			WillReturnResult(sqlmock.NewResult(0, 0))
		assert.NoError(t, CallProcTx(context.Background(), ex, "P_G", 1))
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("nil 与未初始化 gorm.DB 报校验错误", func(t *testing.T) {
		_, err := FromGorm(nil)
		assert.Error(t, err)
		_, err = FromGorm(&gorm.DB{})
		assert.Error(t, err)
		e, ok := fverr.From(err)
		assert.True(t, ok)
		assert.Equal(t, fverr.CodeValidation, e.Code)
	})
}

func TestPlaceholderHelpers(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "procSQL 无参", got: procSQL("P", 0), want: "BEGIN P; END;"},
		{name: "procSQL 三参", got: procSQL("P", 3), want: "BEGIN P(:1, :2, :3); END;"},
		{name: "funcSQL 无参", got: funcSQL("F", 0), want: "BEGIN :1 := F; END;"},
		{name: "funcSQL 两参", got: funcSQL("F", 2), want: "BEGIN :1 := F(:2, :3); END;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.got)
		})
	}
}

// driverRows 断言 fakeDriverRows 满足驱动行集接口。
var _ driver.Rows = fakeDriverRows{}

func TestCallFuncInvalidOutArgAndCursor(t *testing.T) {
	t.Run("Out 非法入参经 CallFunc 返回校验错误", func(t *testing.T) {
		db, mock := procDB(t)
		var v int64
		err := db.CallFunc(context.Background(), "F", &v, Out(42))
		assert.Error(t, err)
		e, ok := fverr.From(err)
		assert.True(t, ok)
		assert.Equal(t, fverr.CodeValidation, e.Code)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("游标经 CallFunc 回填", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		drv := NewMockdriverAdapter(ctrl)
		db, mock := procDB(t)
		db.drv = drv

		rowsSQL, rowsMock := newMockDB(t)
		rowsMock.ExpectQuery("SELECT refcursor").WillReturnRows(
			sqlmock.NewRows([]string{"ID"}).AddRow(9))
		wantRows, err := rowsSQL.QueryContext(context.Background(), "SELECT refcursor")
		assert.NoError(t, err)

		var cur Cursor
		arg := &outCursor{dest: &cur, rows: fakeDriverRows{}}
		var v int64
		mock.ExpectExec("BEGIN :1 := F_CUR(:2); END;").
			WithArgs(outDestMatcher{desc: "*int64", dest: &v}, cursorDestMatcher{}).
			WillReturnResult(sqlmock.NewResult(0, 0))
		drv.EXPECT().NewCursor(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&Cursor{rows: wantRows}, nil)

		assert.NoError(t, db.CallFunc(context.Background(), "F_CUR", &v, arg))
		assert.Same(t, wantRows, cur.rows)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestCallFuncExecError(t *testing.T) {
	db, mock := procDB(t)
	var v int64
	mock.ExpectExec("BEGIN :1 := F_BAD(:2); END;").
		WithArgs(outDestMatcher{desc: "*int64", dest: &v}, sqlmock.AnyArg()).
		WillReturnError(errors.New("ORA-01476"))
	err := db.CallFunc(context.Background(), "F_BAD", &v, 0)
	assert.Error(t, err)
	e, ok := fverr.From(err)
	assert.True(t, ok)
	assert.Equal(t, fverr.CodeDB, e.Code)
	assert.Contains(t, err.Error(), "F_BAD")
	assert.NoError(t, mock.ExpectationsWereMet())
}
