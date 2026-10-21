package server_test

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	server "github.com/fab-viewer/common-lib/cmd/server"
)

func TestNewServer(t *testing.T) {
	cfg := server.Config{
		Addr:              ":9090",
		Mode:              "release",
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ShutdownTimeout:   2 * time.Second,
	}
	engine := server.NewEngine(nil)
	srv, cleanup := server.NewServer(cfg, engine)

	assert.Equal(t, ":9090", srv.Addr)
	assert.Same(t, engine, srv.Handler, "Handler 即传入引擎")
	assert.Equal(t, 5*time.Second, srv.ReadHeaderTimeout)
	assert.Equal(t, 30*time.Second, srv.ReadTimeout)
	assert.Equal(t, 30*time.Second, srv.WriteTimeout)
	assert.Equal(t, 60*time.Second, srv.IdleTimeout)

	// 未监听的 server 调 Shutdown 返回 ErrServerClosed：cleanup 记日志不 panic
	require.NotPanics(t, cleanup)
}

func TestRunListenError(t *testing.T) {
	// 占住端口使 ListenAndServe 立即失败，Run 应返回错误而非阻塞
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	srv := &http.Server{Addr: ln.Addr().String(), Handler: server.NewEngine(nil)}
	err = server.Run(srv)
	require.Error(t, err)
}
