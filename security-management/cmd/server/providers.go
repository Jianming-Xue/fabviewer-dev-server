package main

import (
	"github.com/gin-gonic/gin"

	server "github.com/fab-viewer/common-lib/cmd/server"
	"github.com/fab-viewer/common-lib/common/logx"
	"github.com/fab-viewer/common-lib/storage/oracle/conn"
	"github.com/fab-viewer/security-management/api"
	"github.com/fab-viewer/security-management/domain"
	"github.com/fab-viewer/security-management/services"
	"github.com/fab-viewer/security-management/storage/memory"
	oraclestorage "github.com/fab-viewer/security-management/storage/oracle"
)

// storage 汇聚按环境装配的存储实现：oracle 模式下连接为单例，repo 与
// 事务管理器共用，cleanup 关闭连接；memory 模式无资源可释放。
type storage struct {
	repo domain.UserRepository
	txm  domain.TxManager
}

// provideStorage 按配置装配存储：application.yml 含 oracle 段即启用
// oracle（生产）——组件自治 conn.Init 读段绑定校验建连（敏感值经
// security.oracle.* 环境变量覆盖注入，见 application.yml 样例）；无段
// 回退内存（开发自举，重启即失）。启用与否完全由配置文件表达。
func provideStorage(m map[string]any, log logx.Logger) (st *storage, cleanup func(), err error) {
	db, cleanupDB, err := conn.Init(m, conn.WithLogger(log))
	if err != nil {
		return nil, nil, err
	}
	if db == nil {
		log.Warn("storage=memory（开发自举：无持久化，重启即失；启用 oracle 见 application.yml 的 oracle 段）")
		return &storage{repo: memory.NewUserRepo()}, func() {}, nil
	}
	// WithLogger 已在 Init 透传：GORM SQL 日志（含慢 SQL）汇入统一日志出口
	repo, err := oraclestorage.NewUserRepo(db)
	if err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	return &storage{repo: repo, txm: oraclestorage.NewTxManager(db)}, cleanupDB, nil
}

// provideUserRepo 从汇聚存储取出用户仓储（wire 图中 repo 与事务管理器
// 共用同一 oracle 连接；TxManager 待用户↔角色多写场景接入后再进 wire 图，
// 见 storage/oracle/tx.go 注释）。
func provideUserRepo(st *storage) domain.UserRepository { return st.repo }

// provideRouter 组装引擎与业务路由：四中间件固定顺序与 /healthz 探活由
// server.NewEngine 承载；模块码（bizerror.Code）由 fault 直接透传到
// 响应体 code，无需注册映射。
func provideRouter(users services.UserService) *gin.Engine {
	r := server.NewEngine()
	api.RegisterRoutes(r, users)
	return r
}
