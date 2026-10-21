// Command server 是 security-management 的组装根：装配日志、存储实现、
// 服务实现与路由，并优雅停机。存储实现按环境选择：
//
//	SM_DB_USER 等数据库环境变量齐全 → storage/oracle（生产）
//	缺省                      → storage/memory（开发自举，重启即失）
//
// 启动：
//
//	go run ./cmd/server
//
// 试一下：
//
//	curl -i -X POST -d '{"username":"zhangsan","name":"张三"}' \
//	     -H 'Content-Type: application/json' http://localhost:8081/api/users
//	curl -i http://localhost:8081/api/users?pageIndex=1&pageSize=5
//	curl -i -H 'userName: zhangsan' http://localhost:8081/api/users/{id}
//	curl -i -X DELETE -H 'Accept-Language: zh-CN' http://localhost:8081/api/users/{id}  # 确认消息带姓名
//	curl -i -X DELETE 'http://localhost:8081/api/users/{id}?confirm=true'               # 真正删除
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/fab-viewer/common-lib/common/config"
	"github.com/fab-viewer/common-lib/common/logx"
	oracleconfig "github.com/fab-viewer/common-lib/storage/oracle/config"
	"github.com/fab-viewer/common-lib/storage/oracle/conn"
	"github.com/fab-viewer/security-management/api"
	"github.com/fab-viewer/security-management/app"
	"github.com/fab-viewer/security-management/domain"
	"github.com/fab-viewer/security-management/storage/memory"
	oraclestorage "github.com/fab-viewer/security-management/storage/oracle"
)

func main() {
	// 配置与日志：application.yml（路径可经 security_CFG 环境变量覆盖，
	// 叶子可经 security.* 环境变量覆盖）→ log 段双出口（console+file）
	cfgMap, err := config.LoadConfig("security")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	log, closeLog, err := logx.Init(cfgMap)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer closeLog() // Init 已落位全局出口，包级 logx.Info 直接可用

	userRepo, txm, err := openStorage(log)
	if err != nil {
		log.Error("storage init failed", "err", err)
		os.Exit(1)
	}
	_ = txm // 事务端口在多写场景接入（见 storage/oracle/tx.go 注释）

	userSvc := app.NewUserService(userRepo)
	router := api.NewRouter(log, userSvc)

	addr := envOr("SM_ADDR", ":8081")
	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info("listening", "addr", addr, "storage", storageMode())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server exited", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "err", err)
	}
	log.Info("bye")
	closeLog()
}

// openStorage 按环境装配存储：oracle 环境变量齐全走 Oracle，否则内存。
func openStorage(log logx.Logger) (domain.UserRepository, domain.TxManager, error) {
	if storageMode() == "oracle" {
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
			return nil, nil, err
		}
		return repo, oraclestorage.NewTxManager(db), nil
	}
	log.Warn("storage=memory（开发自举：无持久化，重启即失）")
	return memory.NewUserRepo(), nil, nil
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
