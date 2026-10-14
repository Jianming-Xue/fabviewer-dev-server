package oracle

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/gorm"
	"gorm.io/gorm/utils/tests"

	"github.com/fab-viewer/common-lib/logx"
)

// newMockDB 返回使用精确 SQL 匹配的 sqlmock 连接（避免正则元字符干扰）。
func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("newMockDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, mock
}

// connDialector 把 GORM 装配到既有 *sql.DB 上（注册默认回调后替换连接池），
// 用于在不触发真实驱动的情况下单测 Open。
type connDialector struct {
	tests.DummyDialector
	conn *sql.DB
}

// Initialize 实现 gorm.Dialector：复用 DummyDialector 的回调注册并注入 mock 连接。
func (d connDialector) Initialize(db *gorm.DB) error {
	if err := d.DummyDialector.Initialize(db); err != nil {
		return err
	}
	db.ConnPool = d.conn
	return nil
}

// outDestMatcher 是 sqlmock 自定义参数匹配器：断言绑定值为 sql.Out 且
// Dest 与期望指针同一（验证 Out/OutCursor 到 sql.Out 的翻译）。
type outDestMatcher struct {
	desc string
	dest any
}

// Match 实现 sqlmock.Argument。
func (m outDestMatcher) Match(v driver.Value) bool {
	out, ok := v.(sql.Out)
	if !ok {
		return false
	}
	return out.Dest == m.dest
}

// String 实现 sqlmock.Argument，失败时输出可读信息。
func (m outDestMatcher) String() string { return "sql.Out(dest " + m.desc + ")" }

// cursorDestMatcher 断言绑定值为 sql.Out 且 Dest 为 *driver.Rows
// （OutCursor 的游标通道）。
type cursorDestMatcher struct{}

// Match 实现 sqlmock.Argument。
func (cursorDestMatcher) Match(v driver.Value) bool {
	out, ok := v.(sql.Out)
	if !ok {
		return false
	}
	_, isRows := out.Dest.(*driver.Rows)
	return isRows
}

// String 实现 sqlmock.Argument。
func (cursorDestMatcher) String() string { return "sql.Out(dest *driver.Rows)" }

// fakeDriverRows 模拟驱动写回的 REF CURSOR 行集（空结果）。
type fakeDriverRows struct{}

// Columns 实现 driver.Rows。
func (fakeDriverRows) Columns() []string { return []string{"FAKE"} }

// Close 实现 driver.Rows。
func (fakeDriverRows) Close() error { return nil }

// Next 实现 driver.Rows：恒返回 io.EOF（空游标）。
func (fakeDriverRows) Next(_ []driver.Value) error { return io.EOF }

// panicExecer 的 ExecContext 直接 panic，用于验证包边界 recover 转 error。
type panicExecer struct{}

// ExecContext 实现 Execer，恒 panic。
func (panicExecer) ExecContext(_ context.Context, _ string, _ ...any) (sql.Result, error) {
	panic("boom in bind")
}

// QueryContext 实现 Execer。
func (panicExecer) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, nil
}

// recEntry 记录一次日志调用。
type recEntry struct {
	level string
	msg   string
}

// recLogger 是记录型 logx.Logger，并发安全（-race 下多 goroutine 断言）。
type recLogger struct {
	entries []recEntry
}

func (l *recLogger) Debug(msg string, _ ...any) {
	l.entries = append(l.entries, recEntry{"debug", msg})
}
func (l *recLogger) Info(msg string, _ ...any) { l.entries = append(l.entries, recEntry{"info", msg}) }
func (l *recLogger) Warn(msg string, _ ...any) { l.entries = append(l.entries, recEntry{"warn", msg}) }
func (l *recLogger) Error(msg string, _ ...any) {
	l.entries = append(l.entries, recEntry{"error", msg})
}

// With 实现 logx.Logger：原样返回自身便于断言。
func (l *recLogger) With(_ ...any) logx.Logger { return l }

// levels 返回已记录的级别序列；无记录时返回 nil 便于与期望值直接比对。
func (l *recLogger) levels() []string {
	if len(l.entries) == 0 {
		return nil
	}
	out := make([]string, 0, len(l.entries))
	for _, e := range l.entries {
		out = append(out, e.level)
	}
	return out
}
