package conn

import (
	"context"
	"database/sql/driver"

	"gorm.io/gorm"
)

//go:generate go tool mockgen -source=driver.go -destination=mock_driverAdapter.go -package=conn

// driverAdapter 隔离具体 Oracle 驱动的两处真实差异点：GORM Dialector 构造与
// REF CURSOR OUT 参数的行集包装。默认实现见 driver_godror.go（CGO + Instant
// Client）；未来切换纯 Go 驱动（如 go-ora）时只需新增实现并替换 defaultDriver，
// 业务代码零改动。
type driverAdapter interface {
	// Name 返回驱动标识（日志与诊断用），如 "godror"。
	Name() string
	// GormDialector 基于连接 DSN 构造 GORM Dialector。
	GormDialector(dsn string) gorm.Dialector
	// NewCursor 将存储过程 OUT 绑定写回的驱动行集（godror 经 sql.Out 写入的
	// driver.Rows）包装为面向业务的 Cursor。q 提供游标取数所需的查询通道
	// （*sql.DB / *sql.Tx 等满足 Execer 者）。
	NewCursor(ctx context.Context, q Execer, rows driver.Rows) (*Cursor, error)
}
