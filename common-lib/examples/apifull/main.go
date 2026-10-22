// Command apifull 演示 common-lib API 组件的完整用法（无 wire 的手工
// 全家桶形态）：server.NewEngine 引擎装配（四中间件固定顺序 + /healthz）、
// 请求运行时上下文、统一响应体、分页参数解析、错误消息国际化与优雅停机。
// 业务服务的 wire 形态见 security-management 的 cmd/server。
//
// 启动：
//
//	go run ./examples/apifull
//
// 试一下：
//
//	curl -i 'http://localhost:8080/api/users?pageIndex=2&pageSize=5'
//	curl -i -H 'txId: my-tx-1' -H 'txTime: 20261019173000' \
//	     -H 'txName: getUser' -H 'userName: zhangsan' \
//	     http://localhost:8080/api/users/1
//	curl -i http://localhost:8080/api/users/999                    # 404 默认英文
//	curl -i -H 'Accept-Language: zh-CN' http://localhost:8080/api/users/999  # 中文错误消息
//	curl -i -H 'Accept-Language: zh-CN' http://localhost:8080/api/orders/1  # 第二个模块的中文消息
//	curl -i -X DELETE -H 'Accept-Language: zh-CN' http://localhost:8080/api/users/3  # 删除确认（带用户名）
//	curl -i http://localhost:8080/api/boom        # panic → 500 统一体
//	curl -i http://localhost:8080/healthz
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/common-lib/api/request"
	"github.com/fab-viewer/common-lib/api/response"
	server "github.com/fab-viewer/common-lib/cmd/server"
	"github.com/fab-viewer/common-lib/common/bizerror"
	"github.com/fab-viewer/common-lib/common/logx"
	"github.com/fab-viewer/common-lib/common/paging"
	"github.com/fab-viewer/common-lib/examples/apifull/ordererrors"
	"github.com/fab-viewer/common-lib/examples/apifull/usererrors"
)

// 错误文案是本示例（业务服务）自己的配置：各模块（usererrors / ordererrors）
// 包内按语言写成 Go 文件（messages_en.go / messages_zh.go，编译期直接进包），
// 生成器产出各自的 ErrorCode 字典与构造函数——模块字典跟着模块包走，
// 无需全局注册；创建错误时按请求语言查模板，出口直出 Message 不翻译。

// user 模拟业务对象层的 DTO。
type user struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// userRepo 模拟业务存储层：真实项目中由服务层/聚合层调用，
// 这里用内存数据代替 Oracle/ClickHouse/OpenSearch 访问。
type userRepo struct {
	data []user
}

func (r *userRepo) list(_ context.Context, q paging.PageRequest) (paging.Pageable[user], error) {
	start, end := q.Offset(), q.Offset()+q.Limit()
	if start >= len(r.data) {
		return paging.NewPageable[user](q, int64(len(r.data)), nil), nil
	}
	if end > len(r.data) {
		end = len(r.data)
	}
	return paging.NewPageable[user](q, int64(len(r.data)), r.data[start:end]), nil
}

// get 模拟存储层读单条：NewUserNotFound(ctx) 创建时按请求语言查出模板
// （如 "用户 {0} 不存在"），InsertArgs 按序替换占位符，AttachError 补根因
// 与明细（只进日志）。
func (r *userRepo) get(ctx context.Context, id int64) (user, error) {
	for _, u := range r.data {
		if u.ID == id {
			return u, nil
		}
	}
	return user{}, usererrors.NewUserNotFound(ctx).
		InsertArgs(id).
		AttachError(bizerror.ErrNotFound, map[string]any{"id": id})
}

func main() {
	// 完整日志工具：console + file 双出口（lumberjack 滚动）。示例内联
	// 构造 log 段（等价于 application.yml 的写法），真实业务服务经
	// config.LoadConfig 读文件后交给 logx.Init（cmd/server 统一装配调用）
	log, closeLog, err := logx.Init(map[string]any{"log": map[string]any{
		"console": map[string]any{"enabled": true, "format": "console"},
		"file": map[string]any{
			"enabled": true, "path": "apifull.log",
			"maxSize": 10, "maxBackups": 3,
		},
	}})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// 正常停机路径在 main 末尾显式 closeLog（os.Exit 分支不执行 defer）

	repo := &userRepo{data: seedUsers(57)}

	// server 段缺失全默认：":8080"、release 模式、超时默认（真实业务服务
	// 经 config.LoadConfig 读 application.yml 后传 map 进来）
	cfg, err := server.Init(nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// 引擎装配归 server.NewEngine：四中间件固定顺序 + /healthz 探活；
	// 模块错误码（usererrors/ordererrors 的 bizerror.Code）由 fault 直接
	// 透传到响应体 code，无需注册映射
	r := server.NewEngine()

	apiGroup := r.Group("/api")
	apiGroup.GET("/users", func(c *gin.Context) {
		res, err := repo.list(c.Request.Context(), request.PageQuery(c))
		if err != nil {
			_ = c.Error(err)
			return
		}
		response.OK(c, res)
	})
	apiGroup.GET("/users/:id", func(c *gin.Context) {
		var req struct {
			ID int64 `uri:"id" binding:"required"`
		}
		if err := c.ShouldBindUri(&req); err != nil {
			// 错误处直接调模块构造函数：创建时即按请求语言查出 Message
			_ = c.Error(usererrors.NewInvalidUserID(c.Request.Context()))
			return
		}
		u, err := repo.get(c.Request.Context(), req.ID)
		if err != nil {
			_ = c.Error(err)
			return
		}
		response.OK(c, u)
	})
	apiGroup.GET("/orders/:id", func(c *gin.Context) {
		// 演示多模块：ordererrors 与 usererrors 各自维护语言字典、互不感知
		_ = c.Error(ordererrors.NewOrderNotFound(c.Request.Context()))
	})
	apiGroup.DELETE("/users/:id", func(c *gin.Context) { deleteUser(c, repo) })

	apiGroup.GET("/boom", func(_ *gin.Context) { panic("demo panic") })

	// NewServer 组装 http.Server（cleanup 承载优雅停机）；Run 阻塞运行、
	// 等信号后返回。真实项目用 wire 时 cleanup 链自动串起 HTTP 停机与
	// 各存储客户端 Close（见 security-management 的 cmd/server）。
	srv, cleanup := server.NewServer(cfg, r)
	if err := server.Run(srv); err != nil {
		os.Exit(1) // Run 已记录 server exited 日志
	}
	cleanup() // 等待在途请求结束（上限 cfg.ShutdownTimeout）
	log.Info("bye")
	closeLog()
}

// deleteUser 演示删除确认：确认 Message 必须带上被删对象的 Name（用户名/
// 角色名/登录策略名同理），模板占位符 {0} + InsertArgs(name) 组合，各语言
// 语序不同也不受影响。业务服务（如 security-management 的用户/角色/登录
// 策略删除）照此模式接入。
func deleteUser(c *gin.Context, repo *userRepo) {
	var req struct {
		ID int64 `uri:"id" binding:"required"`
	}
	if err := c.ShouldBindUri(&req); err != nil {
		_ = c.Error(usererrors.NewInvalidUserID(c.Request.Context()))
		return
	}
	u, err := repo.get(c.Request.Context(), req.ID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	if c.Query("confirm") != "true" {
		// 未确认：返回带用户名的确认消息，前端弹确认框
		_ = c.Error(usererrors.NewConfirmDeleteUser(c.Request.Context()).
			InsertArgs(u.Name))
		return
	}
	response.OK(c, nil)
}

func seedUsers(n int) []user {
	users := make([]user, 0, n)
	for i := 1; i <= n; i++ {
		users = append(users, user{ID: int64(i), Name: "user-" + strconv.Itoa(i)})
	}
	return users
}
