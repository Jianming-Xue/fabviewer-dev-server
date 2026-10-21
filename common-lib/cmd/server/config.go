package server

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/fab-viewer/common-lib/common/config"
)

// Config 是 server 配置段（application.yml 的 server 段）。段缺失时全默认；
// 段内出现的键覆盖默认，时长写成 "5s" 式字符串（经 config.Bind 的时长钩子
// 转换）。0 表示禁用对应超时（ShutdownTimeout 为 0 表示停机不等待在途请求）。
type Config struct {
	Addr              string        `mapstructure:"addr"`
	Mode              string        `mapstructure:"mode"` // debug|release|test
	ReadHeaderTimeout time.Duration `mapstructure:"readHeaderTimeout"`
	ReadTimeout       time.Duration `mapstructure:"readTimeout"`
	WriteTimeout      time.Duration `mapstructure:"writeTimeout"`
	IdleTimeout       time.Duration `mapstructure:"idleTimeout"`
	ShutdownTimeout   time.Duration `mapstructure:"shutdownTimeout"`
}

// defaultConfig 返回全默认配置：端口 8080、release 模式（gin 不打印路由表
// 到 stdout）、头部读取 5s / 读写 30s / 空闲 120s、优雅停机等待 10s。
func defaultConfig() Config {
	return Config{
		Addr:              ":8080",
		Mode:              gin.ReleaseMode,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		ShutdownTimeout:   10 * time.Second,
	}
}

// Init 按组件自治约定从配置 map 读 server 段：段缺失全默认，段内出现的
// 键覆盖默认值。成功后设置 gin 运行模式（配置优先于 GIN_MODE 环境变量）。
func Init(m map[string]any) (Config, error) {
	cfg := defaultConfig()
	if sub := config.Get(m, "server"); sub != nil {
		if err := config.Bind(sub, &cfg); err != nil {
			return Config{}, fmt.Errorf("server 段绑定失败: %w", err)
		}
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	gin.SetMode(cfg.Mode)
	return cfg, nil
}

// validate 校验合法性：mode 三态、addr 非空、各时长非负。
func (c Config) validate() error {
	switch c.Mode {
	case gin.DebugMode, gin.ReleaseMode, gin.TestMode:
	default:
		return fmt.Errorf("server.mode 非法: %q（debug|release|test）", c.Mode)
	}
	if c.Addr == "" {
		return fmt.Errorf("server.addr 不能为空")
	}
	if c.ReadHeaderTimeout < 0 || c.ReadTimeout < 0 || c.WriteTimeout < 0 ||
		c.IdleTimeout < 0 || c.ShutdownTimeout < 0 {
		return fmt.Errorf("server 各时长不能为负")
	}
	return nil
}
