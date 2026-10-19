package conn

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
	"gorm.io/gorm/utils/tests"

	"github.com/fab-viewer/common-lib/common/errs"
	"github.com/fab-viewer/common-lib/storage"
	"github.com/fab-viewer/common-lib/storage/oracle/config"
)

// validCfg 返回可通过校验的最小配置。
func validCfg() config.Config {
	return config.Config{User: "u", Password: "p", Host: "h", ServiceName: "s"}
}

// openMock 注入 mock 驱动完成 Open：dialector 指向 sqlmock 连接并预置探活成功。
func openMock(t *testing.T, cfg config.Config, opts ...Option) (*DB, sqlmock.Sqlmock) { //nolint:gocritic // 配置值对象按值传递
	t.Helper()
	sqlDB, mock := newMockDB(t)
	mock.ExpectQuery(pingQuery).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	ctrl := gomock.NewController(t)
	drv := NewMockdriverAdapter(ctrl)
	drv.EXPECT().GormDialector(gomock.Any()).
		DoAndReturn(func(dsn string) gorm.Dialector {
			assert.Contains(t, dsn, `user="u"`, "DSN 应携带用户名")
			assert.Contains(t, dsn, cfg.Host+":"+defaultPortLabel(), "DSN 应携带连接串")
			return connDialector{conn: sqlDB}
		})
	db, err := Open(context.Background(), cfg, append([]Option{withDriver(drv)}, opts...)...)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return db, mock
}

// defaultPortLabel 生成默认端口文本（测试断言用）。
func defaultPortLabel() string { return "1521" }

func TestOpenSuccess(t *testing.T) {
	db, mock := openMock(t, validCfg())
	assert.NotNil(t, db.GORM())
	assert.Equal(t, "dummy", db.GORM().Name())

	sqlDB := db.SQL()
	assert.NotNil(t, sqlDB)
	// 连接池默认值已生效（Pool.WithDefaults；DBStats 仅暴露 MaxOpenConnections）
	assert.Equal(t, 100, sqlDB.Stats().MaxOpenConnections)

	// PrepareStmt 默认启用：连接池被包成 *gorm.PreparedStmtDB
	_, isPrepared := db.GORM().Statement.ConnPool.(*gorm.PreparedStmtDB)
	assert.True(t, isPrepared, "PrepareStmt 默认应启用")

	// 显式 Ping 走同一探活语句且受 ctx 控制
	mock.ExpectQuery(pingQuery).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	assert.NoError(t, db.Ping(context.Background()))

	mock.ExpectClose()
	assert.NoError(t, db.Close())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOpenPrepareStmtDisabled(t *testing.T) {
	sqlDB, mock := newMockDB(t)
	mock.ExpectQuery(pingQuery).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	ctrl := gomock.NewController(t)
	drv := NewMockdriverAdapter(ctrl)
	drv.EXPECT().GormDialector(gomock.Any()).Return(connDialector{conn: sqlDB})
	no := false
	cfg := validCfg()
	cfg.PrepareStmt = &no

	db, err := Open(context.Background(), cfg, withDriver(drv))
	assert.NoError(t, err)
	assert.Same(t, sqlDB, db.SQL())
	// 关闭预编译后 ConnPool 保持原连接池
	assert.Same(t, sqlDB, db.GORM().Statement.ConnPool)
	mock.ExpectClose()
	assert.NoError(t, db.Close())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOpenValidation(t *testing.T) {
	cases := []struct {
		name string
		mut  func(config.Config) config.Config
	}{
		{name: "缺 User", mut: func(c config.Config) config.Config { c.User = ""; return c }},
		{name: "缺 Host", mut: func(c config.Config) config.Config { c.Host = ""; return c }},
		{name: "缺 ServiceName", mut: func(c config.Config) config.Config { c.ServiceName = ""; return c }},
		{name: "LogLevel 非法", mut: func(c config.Config) config.Config { c.LogLevel = config.LogLevel("verbose"); return c }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Open(context.Background(), tt.mut(validCfg()))
			assert.Error(t, err)
			e, ok := errs.From(err)
			assert.True(t, ok)
			assert.Equal(t, errs.CodeValidation, e.Code)
		})
	}
}

func TestOpenPingFailureClosesPool(t *testing.T) {
	sqlDB, mock := newMockDB(t)
	ctrl := gomock.NewController(t)
	drv := NewMockdriverAdapter(ctrl)
	drv.EXPECT().GormDialector(gomock.Any()).Return(connDialector{conn: sqlDB})
	mock.ExpectQuery(pingQuery).WillReturnError(errors.New("ORA-12541"))
	mock.ExpectClose()

	_, err := Open(context.Background(), validCfg(), withDriver(drv))
	assert.Error(t, err)
	e, ok := errs.From(err)
	assert.True(t, ok)
	assert.Equal(t, errs.CodeDB, e.Code)
	assert.Contains(t, err.Error(), "oracle.Ping")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPingUnexpectedValue(t *testing.T) {
	sqlDB, mock := newMockDB(t)
	db := &DB{sqlDB: sqlDB, drv: defaultDriver}
	mock.ExpectQuery(pingQuery).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(2))
	err := db.Ping(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "非预期值")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPingContextCanceled(t *testing.T) {
	db, mock := openMock(t, validCfg())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := db.Ping(ctx)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
	mock.ExpectClose()
	assert.NoError(t, db.Close())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNilDBGuards(t *testing.T) {
	var db *DB
	assert.Error(t, db.Ping(context.Background()))
	assert.NoError(t, db.Close())
}

func TestOpenWithLoggerNilFallback(t *testing.T) {
	db, mock := openMock(t, validCfg(), WithLogger(nil))
	assert.NotNil(t, db)
	mock.ExpectClose()
	assert.NoError(t, db.Close())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOpenAppliesCustomPool(t *testing.T) {
	cfg := validCfg()
	cfg.Pool = storage.PoolConfig{MaxOpenConns: 7, MaxIdleConns: 3,
		ConnMaxLifetime: time.Minute, ConnMaxIdleTime: 30 * time.Second}
	db, mock := openMock(t, cfg)
	stats := db.SQL().Stats()
	assert.Equal(t, 7, stats.MaxOpenConnections)
	mock.ExpectClose()
	assert.NoError(t, db.Close())
	assert.NoError(t, mock.ExpectationsWereMet())
}

// nopPool 满足 gorm.ConnPool 但不提供 *sql.DB，用于触发 gorm.DB() 失败分支。
type nopPool struct{}

// PrepareContext 实现 gorm.ConnPool。
func (nopPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("nop")
}

// ExecContext 实现 gorm.ConnPool。
func (nopPool) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return nil, errors.New("nop")
}

// QueryContext 实现 gorm.ConnPool。
func (nopPool) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, errors.New("nop")
}

// QueryRowContext 实现 gorm.ConnPool。
func (nopPool) QueryRowContext(context.Context, string, ...any) *sql.Row { return nil }

// poolDialector 把 ConnPool 设为 nopPool。
type poolDialector struct {
	tests.DummyDialector
}

// Initialize 实现 gorm.Dialector：注入无 SQL 能力的连接池。
func (poolDialector) Initialize(db *gorm.DB) error {
	if err := (tests.DummyDialector{}).Initialize(db); err != nil {
		return err
	}
	db.ConnPool = nopPool{}
	return nil
}

func TestOpenExtractSQLDBError(t *testing.T) {
	ctrl := gomock.NewController(t)
	drv := NewMockdriverAdapter(ctrl)
	drv.EXPECT().GormDialector(gomock.Any()).Return(poolDialector{})

	no := false
	cfg := validCfg()
	cfg.PrepareStmt = &no
	_, err := Open(context.Background(), cfg, withDriver(drv))
	assert.Error(t, err)
	e, ok := errs.From(err)
	assert.True(t, ok)
	assert.Equal(t, errs.CodeDB, e.Code)
	assert.Contains(t, err.Error(), "提取 *sql.DB")
}

func TestCloseError(t *testing.T) {
	db, mock := openMock(t, validCfg())
	mock.ExpectClose().WillReturnError(errors.New("close refused"))
	err := db.Close()
	assert.Error(t, err)
	e, ok := errs.From(err)
	assert.True(t, ok)
	assert.Equal(t, errs.CodeDB, e.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOpenWithCustomLogger(t *testing.T) {
	db, mock := openMock(t, validCfg(), WithLogger(&recLogger{}))
	assert.NotNil(t, db)
	mock.ExpectClose()
	assert.NoError(t, db.Close())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCloseGorm(t *testing.T) {
	t.Run("nil 直接返回", func(t *testing.T) {
		assert.NotPanics(t, func() { closeGorm(nil) })
	})
	t.Run("关闭底层连接池", func(t *testing.T) {
		sqlDB, mock := newMockDB(t)
		gdb, err := gorm.Open(connDialector{conn: sqlDB}, &gorm.Config{DisableAutomaticPing: true})
		assert.NoError(t, err)
		mock.ExpectClose()
		closeGorm(gdb)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("无法提取连接池时不 panic", func(t *testing.T) {
		gdb, err := gorm.Open(poolDialector{}, &gorm.Config{DisableAutomaticPing: true})
		assert.NoError(t, err)
		assert.NotPanics(t, func() { closeGorm(gdb) })
	})
}
