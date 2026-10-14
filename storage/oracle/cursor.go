package oracle

import (
	"database/sql"
	"database/sql/driver"

	"github.com/fab-viewer/common-lib/errs"
)

// Cursor 包装存储过程 OUT 参数返回的 REF CURSOR，读取语义对齐 *sql.Rows
// （Next/Scan/Columns/Err/Close），内部不暴露任何驱动类型。
// 用完必须 Close 以释放服务端游标；Close 后重复调用安全（幂等）。
type Cursor struct {
	rows *sql.Rows
}

// Next 推进到下一行，返回是否还有数据；发生错误时返回 false，
// 具体错误经 Err 获取。未初始化（零值）的 Cursor 恒返回 false。
func (c *Cursor) Next() bool {
	if c == nil || c.rows == nil {
		return false
	}
	return c.rows.Next()
}

// Scan 把当前行复制到 dest（语义同 *sql.Rows.Scan）。
// 未初始化或已耗尽的 Cursor 调用返回错误而非 panic。
func (c *Cursor) Scan(dest ...any) error {
	if c == nil || c.rows == nil {
		return errs.New(errs.CodeValidation, "oracle.Cursor.Scan 游标未初始化")
	}
	if err := c.rows.Scan(dest...); err != nil {
		return errs.Wrap(err, errs.CodeDB, "oracle.Cursor.Scan")
	}
	return nil
}

// Columns 返回当前结果集的列名列表。
func (c *Cursor) Columns() ([]string, error) {
	if c == nil || c.rows == nil {
		return nil, errs.New(errs.CodeValidation, "oracle.Cursor.Columns 游标未初始化")
	}
	cols, err := c.rows.Columns()
	if err != nil {
		return nil, errs.Wrap(err, errs.CodeDB, "oracle.Cursor.Columns")
	}
	return cols, nil
}

// Err 返回迭代期间（不含 Scan）发生的错误；正常结束返回 nil。
func (c *Cursor) Err() error {
	if c == nil || c.rows == nil {
		return errs.New(errs.CodeValidation, "oracle.Cursor.Err 游标未初始化")
	}
	if err := c.rows.Err(); err != nil {
		return errs.Wrap(err, errs.CodeDB, "oracle.Cursor.Err")
	}
	return nil
}

// Close 释放游标及其关联语句资源，幂等；未初始化的 Cursor 直接返回 nil。
func (c *Cursor) Close() error {
	if c == nil || c.rows == nil {
		return nil
	}
	if err := c.rows.Close(); err != nil {
		return errs.Wrap(err, errs.CodeDB, "oracle.Cursor.Close")
	}
	return nil
}

// outCursor 是 OutCursor 产生的绑定标记，执行成功后由驱动适配器把
// 写回的 driver.Rows 包装为 Cursor 并填入 dest。
type outCursor struct {
	dest *Cursor
	rows driver.Rows
	err  error
}

// OutCursor 声明一个 REF CURSOR 类型的 OUT 参数，dest 在调用成功后被填充：
//
//	var cur oracle.Cursor
//	err := db.CallProc(ctx, "P_LIST", oracle.OutCursor(&cur))
//	defer cur.Close()
//	for cur.Next() { ... cur.Scan(&a, &b) ... }
//
// dest 为 nil 时标记为非法，CallProc 阶段返回错误（不会 panic）。
// 这是全包唯一真正的驱动差异点，翻译逻辑见 proc.go 与 driverAdapter。
func OutCursor(dest *Cursor) (out any) {
	defer func() {
		if r := recover(); r != nil {
			out = &outCursor{err: errs.Newf(errs.CodeValidation, "oracle.OutCursor 入参非法引发 panic: %v", r)}
		}
	}()
	if dest == nil {
		return &outCursor{err: errs.Newf(errs.CodeValidation, "oracle.OutCursor dest 不能为 nil")}
	}
	return &outCursor{dest: dest}
}
