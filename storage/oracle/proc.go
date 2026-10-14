package oracle

import (
	"context"
	"database/sql"
	"reflect"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/fab-viewer/common-lib/errs"
)

// 编译期断言：标准库三种执行器天然满足 Execer。
var (
	_ Execer = (*sql.DB)(nil)
	_ Execer = (*sql.Tx)(nil)
	_ Execer = (*sql.Conn)(nil)
)

// Execer 是存储过程/函数调用所需的最小执行接口：*sql.DB、*sql.Tx、*sql.Conn
// 直接满足；*gorm.DB 无 ExecContext 方法，经 FromGorm 提取当前连接
// （事务回调内即事务连接）后参与调用，从而与 GORM DML 共享同一事务。
type Execer interface {
	// ExecContext 执行无结果集语句（PL/SQL 匿名块）。
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	// QueryContext 提供游标取数通道（driverAdapter.NewCursor 使用）。
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// CallProc 在自有连接池上调用存储过程，生成
// `BEGIN NAME(:1, :2, ...); END;` 按位置绑定。
// IN 参数直接传值；标量 OUT 参数用 Out 包装；游标 OUT 用 OutCursor 包装。
func (db *DB) CallProc(ctx context.Context, name string, args ...any) error {
	return callProc(newCallCtx(ctx, db.drv, db.sqlDB, name), args)
}

// CallFunc 在自有连接池上调用存储函数，生成
// `BEGIN :1 := NAME(:2, ...); END;`，返回值写入 dest（必须为非 nil 指针）。
func (db *DB) CallFunc(ctx context.Context, name string, dest any, args ...any) error {
	return callFunc(newCallCtx(ctx, db.drv, db.sqlDB, name), dest, args)
}

// CallProcTx 在给定执行器（*sql.Tx / *sql.Conn / FromGorm 提取的 GORM 事务连接）
// 上调用存储过程，使其与调用方处于同一事务。
func CallProcTx(ctx context.Context, tx Execer, name string, args ...any) error {
	if tx == nil {
		return errs.Newf(errs.CodeValidation, "oracle.CallProcTx tx 不能为 nil, proc=%s", name)
	}
	return callProc(newCallCtx(ctx, defaultDriver, tx, name), args)
}

// CallFuncTx 在给定执行器上调用存储函数，dest 语义同 CallFunc。
func CallFuncTx(ctx context.Context, tx Execer, name string, dest any, args ...any) error {
	if tx == nil {
		return errs.Newf(errs.CodeValidation, "oracle.CallFuncTx tx 不能为 nil, func=%s", name)
	}
	return callFunc(newCallCtx(ctx, defaultDriver, tx, name), dest, args)
}

// callCtx 聚合过程/函数调用的公共依赖，控制内部函数参数个数（revive argument-limit）。
type callCtx struct {
	ctx  context.Context
	drv  driverAdapter
	ex   Execer
	name string
}

func newCallCtx(ctx context.Context, drv driverAdapter, ex Execer, name string) callCtx {
	return callCtx{ctx: ctx, drv: drv, ex: ex, name: name}
}

// FromGorm 从 *gorm.DB 提取当前连接池（事务回调内为事务连接）作为 Execer，
// 使存储过程调用与 GORM DML 进入同一事务：
//
//	err := gormDB.Transaction(func(tx *gorm.DB) error {
//	    ex, err := oracle.FromGorm(tx)
//	    if err != nil { return err }
//	    return oracle.CallProcTx(ctx, ex, "P_ACCRUE", arg)
//	})
func FromGorm(db *gorm.DB) (Execer, error) {
	if db == nil || db.Statement == nil || db.Statement.ConnPool == nil {
		return nil, errs.New(errs.CodeValidation, "oracle.FromGorm gorm.DB 未初始化")
	}
	// gorm.ConnPool 接口本身包含 ExecContext/QueryContext，必然满足 Execer。
	return db.Statement.ConnPool, nil
}

// outArg 是 Out 产生的标量 OUT 绑定标记，翻译为 sql.Out{Dest: dest}。
type outArg struct {
	dest any
	err  error
}

// Out 声明一个标量 OUT 参数，dest 必须为非 nil 指针（驱动将把 OUT 值写入其中）：
//
//	var total int64
//	err := db.CallProc(ctx, "P_SETTLE", "IN1", oracle.Out(&total))
//
// 非法入参（nil、非指针）在 CallProc/CallFunc 阶段返回错误，不会 panic。
func Out(dest any) (out any) {
	defer func() {
		if r := recover(); r != nil {
			out = outArg{err: errs.Newf(errs.CodeValidation, "oracle.Out 入参非法引发 panic: %v", r)}
		}
	}()
	if dest == nil || reflect.ValueOf(dest).Kind() != reflect.Pointer {
		return outArg{err: errs.Newf(errs.CodeValidation, "oracle.Out dest 必须为非 nil 指针, got %T", dest)}
	}
	return outArg{dest: dest}
}

// callProc 是过程调用核心：标记翻译 → 生成 PL/SQL → 执行 → 游标回填。
func callProc(c callCtx, args []any) error {
	binds, cursors, err := buildBindings(args)
	if err != nil {
		return err
	}
	if err := safeExec(c.ctx, c.ex, procSQL(c.name, len(binds)), binds...); err != nil {
		return err
	}
	return fillCursors(c.ctx, c.drv, c.ex, cursors, c.name)
}

// callFunc 是函数调用核心：返回值作为 :1 号 OUT 绑定，其余参数顺延。
func callFunc(c callCtx, dest any, args []any) error {
	if dest == nil || reflect.ValueOf(dest).Kind() != reflect.Pointer {
		return errs.Newf(errs.CodeValidation, "oracle.CallFunc %s dest 必须为非 nil 指针, got %T", c.name, dest)
	}
	binds, cursors, err := buildBindings(args)
	if err != nil {
		return err
	}
	query := funcSQL(c.name, len(binds))
	binds = append([]any{sql.Out{Dest: dest}}, binds...)
	if err := safeExec(c.ctx, c.ex, query, binds...); err != nil {
		return err
	}
	return fillCursors(c.ctx, c.drv, c.ex, cursors, c.name)
}

// buildBindings 把公开标记（Out/OutCursor 产物）翻译为 database/sql 绑定值：
// outArg → sql.Out{Dest}，outCursor → sql.Out{Dest: *driver.Rows}；
// 其余参数原样透传。同时收集游标标记供执行后回填。
func buildBindings(args []any) (binds []any, cursors []*outCursor, err error) {
	binds = make([]any, 0, len(args))
	for _, arg := range args {
		switch a := arg.(type) {
		case outArg:
			if a.err != nil {
				return nil, nil, a.err
			}
			binds = append(binds, sql.Out{Dest: a.dest})
		case *outCursor:
			if a.err != nil {
				return nil, nil, a.err
			}
			binds = append(binds, sql.Out{Dest: &a.rows})
			cursors = append(cursors, a)
		default:
			binds = append(binds, arg)
		}
	}
	return binds, cursors, nil
}

// fillCursors 把执行成功后驱动写回的 REF CURSOR 行集包装为 Cursor 填入目标；
// 未写回（rows 为 nil，如无驱动支持的模拟环境）时静默跳过。
func fillCursors(ctx context.Context, drv driverAdapter, ex Execer, cursors []*outCursor, name string) error {
	for _, c := range cursors {
		if c.rows == nil {
			continue
		}
		cur, err := drv.NewCursor(ctx, ex, c.rows)
		if err != nil {
			return errs.Wrapf(err, errs.CodeDB, "oracle 游标 OUT 绑定 %s", name)
		}
		*c.dest = *cur
	}
	return nil
}

// safeExec 执行 PL/SQL 块，并在驱动绑定反射链 panic（如非法 dest 类型穿透）
// 时转为业务错误，保证包边界不向调用方抛 panic。
func safeExec(ctx context.Context, ex Execer, query string, args ...any) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errs.Newf(errs.CodeValidation, "oracle 执行 %q 时绑定入参引发 panic: %v", query, r)
		}
	}()
	if _, execErr := ex.ExecContext(ctx, query, args...); execErr != nil {
		return errs.Wrapf(execErr, errs.CodeDB, "oracle 执行 %q 失败", query)
	}
	return nil
}

// procSQL 生成过程调用 PL/SQL 匿名块：BEGIN NAME(:1, :2); END;
func procSQL(name string, n int) string {
	if n <= 0 {
		return "BEGIN " + name + "; END;"
	}
	return "BEGIN " + name + "(" + placeholders(1, n) + "); END;"
}

// funcSQL 生成函数调用 PL/SQL 匿名块（n 为参数个数，不含返回值）：
// BEGIN :1 := NAME(:2, :3); END;
func funcSQL(name string, n int) string {
	if n <= 0 {
		return "BEGIN :1 := " + name + "; END;"
	}
	return "BEGIN :1 := " + name + "(" + placeholders(2, n+1) + "); END;"
}

// placeholders 生成 ":from, :from+1, ..., :to" 占位符串。
func placeholders(from, to int) string {
	parts := make([]string, 0, to-from+1)
	for i := from; i <= to; i++ {
		parts = append(parts, ":"+strconv.Itoa(i))
	}
	return strings.Join(parts, ", ")
}
