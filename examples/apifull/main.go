// Command apifull 演示 common-lib API 组件的完整用法：四个中间件的推荐注册
// 顺序、统一响应体、分页参数解析与优雅停机。
//
// 启动：
//
//	go run ./examples/apifull
//
// 试一下：
//
//	curl -i 'http://localhost:8080/api/users?page=2&size=5'
//	curl -i -H 'X-Request-Id: my-rid-1' http://localhost:8080/api/users/1
//	curl -i http://localhost:8080/api/users/999   # 404 统一错误体
//	curl -i http://localhost:8080/api/boom        # panic → 500 统一体
//	curl -i http://localhost:8080/healthz
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/common-lib/api"
	"github.com/fab-viewer/common-lib/api/middleware"
	"github.com/fab-viewer/common-lib/errs"
	"github.com/fab-viewer/common-lib/logx"
	"github.com/fab-viewer/common-lib/paging"
)

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

func (r *userRepo) list(_ context.Context, q paging.Query) (paging.Result[user], error) {
	start, end := q.Offset(), q.Offset()+q.Limit()
	if start >= len(r.data) {
		return paging.NewResult[user](q, int64(len(r.data)), nil), nil
	}
	if end > len(r.data) {
		end = len(r.data)
	}
	return paging.NewResult[user](q, int64(len(r.data)), r.data[start:end]), nil
}

func (r *userRepo) get(_ context.Context, id int64) (user, error) {
	for _, u := range r.data {
		if u.ID == id {
			return u, nil
		}
	}
	return user{}, errs.Wrapf(errs.ErrNotFound, errs.CodeNotFound, "user %d not found", id)
}

func main() {
	log := logx.NewSlog(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	logx.SetDefault(log)

	repo := &userRepo{data: seedUsers(57)}

	r := gin.New()
	// 注册顺序固定：AccessLog → ErrorHandler → Recovery → RequestID
	// AccessLog 最外层，保证错误路径记录的 status 是最终值
	r.Use(
		middleware.AccessLog(log),
		middleware.ErrorHandler(log),
		middleware.Recovery(log),
		middleware.RequestID(),
	)

	r.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	apiGroup := r.Group("/api")
	apiGroup.GET("/users", func(c *gin.Context) {
		res, err := repo.list(c.Request.Context(), api.PageQuery(c))
		if err != nil {
			_ = c.Error(err)
			return
		}
		api.OKPage(c, res)
	})
	apiGroup.GET("/users/:id", func(c *gin.Context) {
		var req struct {
			ID int64 `uri:"id" binding:"required"`
		}
		if err := c.ShouldBindUri(&req); err != nil {
			_ = c.Error(errs.Wrap(err, errs.CodeValidation, "参数 id 非法"))
			return
		}
		u, err := repo.get(c.Request.Context(), req.ID)
		if err != nil {
			_ = c.Error(err)
			return
		}
		api.OK(c, u)
	})
	apiGroup.GET("/boom", func(_ *gin.Context) { panic("demo panic") })

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// 优雅停机：SIGINT/SIGTERM 后先 Shutdown HTTP，再按序 Close 各存储客户端
	// （本示例无存储；真实项目此处依次调用 oracle.DB.Close、clickhouse.Client.Close 等）。
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info("listening", "addr", srv.Addr)
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
}

func seedUsers(n int) []user {
	users := make([]user, 0, n)
	for i := 1; i <= n; i++ {
		users = append(users, user{ID: int64(i), Name: "user-" + strconv.Itoa(i)})
	}
	return users
}
