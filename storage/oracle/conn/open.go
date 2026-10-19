package conn

import (
	"context"
	"database/sql"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"

	"github.com/fab-viewer/common-lib/common/errs"
	"github.com/fab-viewer/common-lib/common/logx"
	"github.com/fab-viewer/common-lib/storage/oracle/config"
)

// pingQuery 是连通性探活的 Oracle 专用语法的空表探测语句。
const pingQuery = "SELECT 1 FROM DUAL"

// prepareStmtEnabled 报告是否启用 GORM 预编译语句缓存，nil 视为启用。
func prepareStmtEnabled(v *bool) bool { return v == nil || *v }

// openOptions 聚合 Open 可注入项，均带默认值。
type openOptions struct {
	log logx.Logger
	drv driverAdapter
}

// Option 定制 Open 行为；nil Option 被忽略。
type Option func(*openOptions)

// WithLogger 设置库内日志出口（GORM SQL 日志经 gormlog 适配到此），
// nil 时回退 logx.Default()。
func WithLogger(log logx.Logger) Option {
	return func(o *openOptions) {
		if log != nil {
			o.log = log
		}
	}
}

// withDriver 注入替代驱动适配器（测试注入 mock 用，非公开 API）。
func withDriver(drv driverAdapter) Option {
	return func(o *openOptions) {
		if drv != nil {
			o.drv = drv
		}
	}
}

// DB 是 Oracle 访问封装：聚合 GORM（ORM 出口）与 database/sql（原生 SQL 出口），
// 并提供受 ctx 控制的连通性探活与存储过程调用。非并发独占——所有方法可被
// 多 goroutine 共用，Close 后不可再使用。
type DB struct {
	gormDB *gorm.DB
	sqlDB  *sql.DB
	drv    driverAdapter
}

// Open 初始化 Oracle 连接池并完成 GORM 装配：
// Dialector 经 driverAdapter 构造、GORM logger 经 gormlog 适配到 logx、
// PrepareStmt 默认启用（Config 可关）、连接池取 Pool.WithDefaults 默认值，
// 最后用 SELECT 1 FROM DUAL 在给定 ctx 下探活（gorm 内建的无 ctx 自动 Ping
// 已通过 DisableAutomaticPing 关闭）。ctx 超时/取消会中止建连与探活。
func Open(ctx context.Context, cfg config.Config, opts ...Option) (*DB, error) { //nolint:gocritic // 配置值对象按值传递保证不可变语义
	o := openOptions{log: logx.Default(), drv: defaultDriver}
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	cfg = cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	gormDB, err := gorm.Open(o.drv.GormDialector(cfg.BuildDSN()), &gorm.Config{
		PrepareStmt:          prepareStmtEnabled(cfg.PrepareStmt),
		DisableAutomaticPing: true,
		// Oracle 无引号标识符一律大写存储；GORM 默认 snake_case 小写加引号
		// 生成 "id" 与建表 ID 不匹配（ORA-00904，真实库集成测试抓出），
		// 整体大写 Namer 对齐标准 Oracle DDL，业务模型无需逐字段写 tag。
		NamingStrategy: upperNamer{inner: schema.NamingStrategy{}},
		Logger:         newGormLogger(o.log, cfg.LogLevel, time.Duration(cfg.SlowSQLMs)*time.Millisecond),
	})
	if err != nil {
		return nil, errs.Wrapf(err, errs.CodeDB, "oracle.Open user=%s connectString=%s", cfg.User, cfg.ConnectString())
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		closeGorm(gormDB)
		return nil, errs.Wrap(err, errs.CodeDB, "oracle.Open 提取 *sql.DB")
	}
	sqlDB.SetMaxOpenConns(cfg.Pool.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.Pool.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.Pool.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.Pool.ConnMaxIdleTime)

	db := &DB{gormDB: gormDB, sqlDB: sqlDB, drv: o.drv}
	if err := db.Ping(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return db, nil
}

// GORM 返回 ORM 出口。业务方在查询链上配合 WithContext(ctx) 传播取消信号。
func (db *DB) GORM() *gorm.DB { return db.gormDB }

// SQL 返回原生 SQL 出口（*sql.DB，与 GORM 共享同一连接池）。
func (db *DB) SQL() *sql.DB { return db.sqlDB }

// Ping 用 SELECT 1 FROM DUAL 探活，受 ctx 控制并及时返回底层错误。
func (db *DB) Ping(ctx context.Context) error {
	if db == nil || db.sqlDB == nil {
		return errs.New(errs.CodeValidation, "oracle.Ping db 未初始化")
	}
	var one int
	if err := db.sqlDB.QueryRowContext(ctx, pingQuery).Scan(&one); err != nil {
		return errs.Wrapf(err, errs.CodeDB, "oracle.Ping %s", pingQuery)
	}
	if one != 1 {
		return errs.Newf(errs.CodeDB, "oracle.Ping %s 返回非预期值 %d", pingQuery, one)
	}
	return nil
}

// Close 关闭底层连接池（GORM 与原生 SQL 共享，关闭一次即全部释放）。
func (db *DB) Close() error {
	if db == nil || db.sqlDB == nil {
		return nil
	}
	if err := db.sqlDB.Close(); err != nil {
		return errs.Wrap(err, errs.CodeDB, "oracle.Close")
	}
	return nil
}

// closeGorm 在错误路径上兜底关闭 GORM 持有的连接池。
func closeGorm(gormDB *gorm.DB) {
	if gormDB == nil {
		return
	}
	if sqlDB, err := gormDB.DB(); err == nil && sqlDB != nil {
		_ = sqlDB.Close()
	}
}
