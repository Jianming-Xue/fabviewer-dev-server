package server

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/common-lib/common/logx"
)

// NewServer 按 Config 组装 http.Server。返回的 cleanup 承载优雅停机
// （wire cleanup 链自动调用，顺序在先初始化组件的 cleanup 之前）：等待
// 在途请求结束，上限 ShutdownTimeout（0 表示立即关闭、不等在途请求），
// 超时只记日志不报错。
func NewServer(cfg Config, engine *gin.Engine) (srv *http.Server, cleanup func()) {
	srv = &http.Server{
		Addr:              cfg.Addr,
		Handler:           engine,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
	cleanup = func() {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			logx.Error("graceful shutdown failed", "err", err)
		}
	}
	return srv, cleanup
}

// Run 阻塞运行：监听并在收到 SIGINT/SIGTERM 后返回 nil（停机交给
// NewServer 的 cleanup 执行）；监听失败（非 ErrServerClosed）记录日志并
// 返回错误。生命周期日志经 logx 全局出口记录。
func Run(srv *http.Server) error {
	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	logx.Info("listening", "addr", srv.Addr)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		if err != nil {
			logx.Error("server exited", "err", err)
		}
		return err
	case <-ctx.Done():
		logx.Info("shutting down")
		return nil
	}
}
