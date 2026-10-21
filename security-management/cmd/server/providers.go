package main

import (
	"context"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"

	server "github.com/fab-viewer/common-lib/cmd/server"
	"github.com/fab-viewer/common-lib/common/logx"
	"github.com/fab-viewer/security-management/api"
	"github.com/fab-viewer/security-management/domain"
	"github.com/fab-viewer/security-management/services"
	"github.com/fab-viewer/security-management/storage/memory"
	oraclestorage "github.com/fab-viewer/security-management/storage/oracle"
	"github.com/fab-viewer/security-management/usererrors"
	oracleconfig "github.com/fab-viewer/common-lib/storage/oracle/config"
	"github.com/fab-viewer/common-lib/storage/oracle/conn"
)

// storage 汇聚按环境装配的存储实现：oracle 模式下连接为单例，repo 与
// 事务管理器共用，cleanup 关闭连接；memory 模式无资源可释放。
type storage struct {
	repo domain.UserRepository
	txm  domain.TxManager
}

// provideStorage 按环境装配存储：SM_DB_USER 等数据库环境变量齐全走
// oracle（生产），否则内存（开发自举，重启即失）。
func provideStorage(log logx.Logger) (st *storage, cleanup func(), err error) {
	if storageMode() == "memory" {
		log.Warn("storage=memory（开发自举：无持久化，重启即失）")
		return &storage{repo: memory.NewUserRepo()}, func() {}, nil
	}

	port, _ := strconv.Atoi(envOr("SM_DB_PORT", "1521"))
	cfg := oracleconfig.Config{
		User:        os.Getenv("SM_DB_USER"),
		Password:    os.Getenv("SM_DB_PASSWORD"),
		Host:        os.Getenv("SM_DB_HOST"),
		Port:        port,
		ServiceName: os.Getenv("SM_DB_SERVICE"),
	}.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}
	// WithLogger 把 GORM SQL 日志（含慢 SQL）汇入统一日志出口
	db, err := conn.Open(context.Background(), cfg, conn.WithLogger(log))
	if err != nil {
		return nil, nil, err
	}
	repo, err := oraclestorage.NewUserRepo(db)
	if err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	return &storage{repo: repo, txm: oraclestorage.NewTxManager(db)},
		func() { _ = db.Close() }, nil
}

// provideUserRepo 从汇聚存储取出用户仓储（wire 图中 repo 与事务管理器
// 共用同一 oracle 连接；TxManager 待用户↔角色多写场景接入后再进 wire 图，
// 见 storage/oracle/tx.go 注释）。
func provideUserRepo(st *storage) domain.UserRepository { return st.repo }

// provideRouter 组装引擎与业务路由：四中间件固定顺序与 /healthz 探活由
// server.NewEngine 承载，模块错误码表是业务唯一必传差异。
func provideRouter(users services.UserService) *gin.Engine {
	r := server.NewEngine(usererrors.CodeMap())
	api.RegisterRoutes(r, users)
	return r
}

// storageMode 报告当前存储模式：oracle|memory。
func storageMode() string {
	if os.Getenv("SM_DB_USER") != "" {
		return "oracle"
	}
	return "memory"
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
