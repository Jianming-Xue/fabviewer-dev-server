package conn

import (
	"context"
	"database/sql/driver"

	"github.com/godror/godror"
	gormoracle "github.com/oracle-samples/gorm-oracle/oracle"
	"gorm.io/gorm"

	"github.com/fab-viewer/common-lib/common/fverr"
)

// godrorDriver 是基于 godror（CGO，运行期依赖 Oracle Instant Client）的默认驱动适配器。
// REF CURSOR 经 sql.Out{Dest: *driver.Rows} 绑定写回，再由 godror.WrapRows 包装为
// 标准 *sql.Rows（借道连接池拦截取数，不产生真实数据库往返），
// 以复用 database/sql 的取数与类型转换。
type godrorDriver struct{}

// 编译期断言 godrorDriver 实现 driverAdapter。
var _ driverAdapter = (*godrorDriver)(nil)

// defaultDriver 是包内默认驱动适配器，测试中可经内部 Option 注入 mock 替换。
var defaultDriver driverAdapter = godrorDriver{}

// Name 实现 driverAdapter，返回驱动标识 "godror"。
func (godrorDriver) Name() string { return "godror" }

// GormDialector 实现 driverAdapter，返回 oracle-samples/gorm-oracle 的 Dialector。
func (godrorDriver) GormDialector(dsn string) gorm.Dialector {
	return gormoracle.Open(dsn)
}

// NewCursor 实现 driverAdapter，按 godror 官方推荐用法包装 REF CURSOR。
func (godrorDriver) NewCursor(ctx context.Context, q Execer, rows driver.Rows) (*Cursor, error) {
	if q == nil {
		return nil, fverr.NewError(fverr.CodeValidation, "oracle.NewCursor q 不能为 nil")
	}
	if rows == nil {
		return nil, fverr.NewError(fverr.CodeValidation, "oracle.NewCursor rows 不能为 nil")
	}
	sqlRows, err := godror.WrapRows(ctx, q, rows)
	if err != nil {
		return nil, fverr.NewError(fverr.CodeDB, "oracle.NewCursor godror.WrapRows").AttachError(err, nil)
	}
	return &Cursor{rows: sqlRows}, nil
}
