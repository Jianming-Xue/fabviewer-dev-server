// Command oracleproc 演示 common-lib Oracle 封装的完整用法：
// Open 探活 → GORM CRUD → 存储过程标量 OUT → 存储过程游标 OUT → 优雅 Close。
//
// 本示例自建演示表与存储过程，可反复运行。
//
// 用法（缺少环境变量时仅打印本说明并以 0 退出，CI 只编译不连库也能通过）：
//
//	COMMON_LIB_ORACLE_DSN='user/password@dbhost:1521/ORCLPDB1' go run ./examples/oracleproc
//
// DSN 格式为 user/password@host[:port]/service（缺省端口 1521），
// 运行机器需安装 Oracle Instant Client（godror 为 CGO 驱动）。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fab-viewer/common-lib/errs"
	"github.com/fab-viewer/common-lib/logx"
	"github.com/fab-viewer/common-lib/storage/oracle"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "oracleproc: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	dsn := os.Getenv("COMMON_LIB_ORACLE_DSN")
	if dsn == "" {
		fmt.Print(usage)
		return nil
	}
	cfg, err := parseDSN(dsn)
	if err != nil {
		return err
	}

	log := logx.NewSlog(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	db, err := oracle.Open(ctx, cfg, oracle.WithLogger(log))
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	log.Info("connected", "host", cfg.Host, "service", cfg.ServiceName)

	if err := bootstrap(ctx, db); err != nil {
		return err
	}
	if err := demoGormCRUD(ctx, db, log); err != nil {
		return err
	}
	return demoProc(ctx, db, log)
}

const usage = `
未设置 COMMON_LIB_ORACLE_DSN，跳过 Oracle 演示（exit 0）。

用法：
  COMMON_LIB_ORACLE_DSN='user/password@dbhost:1521/ORCLPDB1' go run ./examples/oracleproc

演示内容：
  1. oracle.Open            建池 + SELECT 1 FROM DUAL 探活
  2. GORM CRUD              建表/插入/查询/更新/删除
  3. oracle.CallProc 标量 OUT  存储过程聚合结果回填
  4. oracle.CallProc 游标 OUT  SYS_REFCURSOR 行集遍历
  5. oracle.DB.Close        释放连接池

前提：运行机器已安装 Oracle Instant Client（godror 为 CGO 驱动）。
`

// parseDSN 解析 user/password@host[:port]/service 形式的演示 DSN。
func parseDSN(dsn string) (oracle.Config, error) {
	at := strings.LastIndex(dsn, "@")
	if at <= 0 || at == len(dsn)-1 {
		return oracle.Config{}, errs.Newf(errs.CodeValidation,
			"DSN 应为 user/password@host[:port]/service 格式, got %q", dsn)
	}
	cred, addr := dsn[:at], dsn[at+1:]
	slash := strings.Index(addr, "/")
	if slash <= 0 {
		return oracle.Config{}, errs.Newf(errs.CodeValidation,
			"DSN 缺少 /service 段, got %q", dsn)
	}
	hostPort, service := addr[:slash], addr[slash+1:]
	host, portStr, hasPort := strings.Cut(hostPort, ":")
	port := oracle.DefaultPort
	if hasPort {
		p, err := strconv.Atoi(portStr)
		if err != nil || p <= 0 {
			return oracle.Config{}, errs.Newf(errs.CodeValidation,
				"DSN 端口非法: %q", portStr)
		}
		port = p
	}
	user, password, ok := strings.Cut(cred, "/")
	if !ok || user == "" || password == "" {
		return oracle.Config{}, errs.Newf(errs.CodeValidation,
			"DSN 凭据应为 user/password 格式, got %q", cred)
	}
	cfg := oracle.Config{
		User:        user,
		Password:    password,
		Host:        host,
		Port:        port,
		ServiceName: service,
		SessionParams: map[string]string{
			"NLS_DATE_FORMAT": "YYYY-MM-DD HH24:MI:SS",
		},
	}
	return cfg, nil
}

// bootstrap 自建演示表与存储过程（幂等：先删旧对象）。
func bootstrap(ctx context.Context, db *oracle.DB) error {
	stmts := []string{
		`BEGIN
  EXECUTE IMMEDIATE 'DROP TABLE CL_DEMO_GADGETS';
EXCEPTION
  WHEN OTHERS THEN
    IF SQLCODE != -942 THEN RAISE; END IF;
END;`,
		`CREATE TABLE CL_DEMO_GADGETS (
  ID   NUMBER(19) PRIMARY KEY,
  NAME VARCHAR2(100) NOT NULL
)`,
		`CREATE OR REPLACE PROCEDURE CL_DEMO_SETTLE(p_factor IN NUMBER, p_total OUT NUMBER) IS
BEGIN
  SELECT NVL(SUM(ID * p_factor), 0) INTO p_total FROM CL_DEMO_GADGETS;
END CL_DEMO_SETTLE;`,
		`CREATE OR REPLACE PROCEDURE CL_DEMO_LIST(p_cur OUT SYS_REFCURSOR) IS
BEGIN
  OPEN p_cur FOR SELECT ID, NAME FROM CL_DEMO_GADGETS ORDER BY ID;
END CL_DEMO_LIST;`,
	}
	for _, s := range stmts {
		if _, err := db.SQL().ExecContext(ctx, s); err != nil {
			return errs.Wrapf(err, errs.CodeDB, "bootstrap 建演示对象失败: %.60s", s)
		}
	}
	return nil
}

// gadget 是 GORM 演示模型（对象层类型的示范：不泄漏 *gorm.DB）。
type gadget struct {
	ID   int64
	Name string
}

// TableName 固定演示表名。
func (gadget) TableName() string { return "CL_DEMO_GADGETS" }

// demoGormCRUD 演示 GORM 出口的增删改查（记得 WithContext 传播取消信号）。
func demoGormCRUD(ctx context.Context, db *oracle.DB, log logx.Logger) error {
	g := db.GORM().WithContext(ctx)

	for i := int64(1); i <= 3; i++ {
		if err := g.Create(&gadget{ID: i, Name: "gadget-" + strconv.FormatInt(i, 10)}).Error; err != nil {
			return errs.Wrap(err, errs.CodeDB, "GORM Create")
		}
	}
	var total int64
	if err := g.Model(&gadget{}).Count(&total).Error; err != nil {
		return errs.Wrap(err, errs.CodeDB, "GORM Count")
	}
	log.Info("gorm create", "rows", total)

	var got gadget
	if err := g.Where("ID = ?", int64(2)).First(&got).Error; err != nil {
		if errors.Is(err, errs.ErrNotFound) {
			return errs.Wrap(err, errs.CodeDB, "GORM First ID=2 不应缺失")
		}
		return errs.Wrap(err, errs.CodeDB, "GORM First ID=2")
	}
	log.Info("gorm first", "id", got.ID, "name", got.Name)

	if err := g.Model(&got).Update("NAME", "gadget-2-updated").Error; err != nil {
		return errs.Wrap(err, errs.CodeDB, "GORM Update ID=2")
	}
	if err := g.Delete(&gadget{}, 3).Error; err != nil {
		return errs.Wrap(err, errs.CodeDB, "GORM Delete ID=3")
	}
	log.Info("gorm update+delete done")
	return nil
}

// demoProc 演示存储过程：标量 OUT（CallProc + Out）与游标 OUT（OutCursor + Cursor）。
func demoProc(ctx context.Context, db *oracle.DB, log logx.Logger) error {
	var total int64
	if err := db.CallProc(ctx, "CL_DEMO_SETTLE", 10, oracle.Out(&total)); err != nil {
		return err
	}
	log.Info("callproc scalar out", "factor", 10, "total", total)

	var cur oracle.Cursor
	if err := db.CallProc(ctx, "CL_DEMO_LIST", oracle.OutCursor(&cur)); err != nil {
		return err
	}
	defer func() { _ = cur.Close() }()

	cols, err := cur.Columns()
	if err != nil {
		return err
	}
	log.Info("callproc cursor out", "columns", cols)
	for cur.Next() {
		var id int64
		var name string
		if err := cur.Scan(&id, &name); err != nil {
			return err
		}
		log.Info("cursor row", "id", id, "name", name)
	}
	return cur.Err()
}
