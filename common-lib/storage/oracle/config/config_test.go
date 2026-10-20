package config

import (
	"testing"
	"time"

	"github.com/godror/godror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/common/fverr"
	"github.com/fab-viewer/common-lib/storage"
)

func TestConfigWithDefaults(t *testing.T) {
	tests := []struct {
		name string
		in   Config
		want Config
	}{
		{
			name: "零值取全默认",
			in:   Config{},
			want: Config{
				Port:      DefaultPort,
				SlowSQLMs: DefaultSlowSQLMs,
				LogLevel:  LogLevelWarn,
				Pool: storage.PoolConfig{
					MaxOpenConns:    100,
					MaxIdleConns:    10,
					ConnMaxLifetime: time.Hour,
					ConnMaxIdleTime: 5 * time.Minute,
				},
			},
		},
		{
			name: "非法值回退默认",
			in: Config{
				Port:      -1,
				SlowSQLMs: -5,
				Pool:      storage.PoolConfig{MaxOpenConns: -3},
			},
			want: Config{
				Port:      DefaultPort,
				SlowSQLMs: DefaultSlowSQLMs,
				LogLevel:  LogLevelWarn,
				Pool: storage.PoolConfig{
					MaxOpenConns:    100,
					MaxIdleConns:    10,
					ConnMaxLifetime: time.Hour,
					ConnMaxIdleTime: 5 * time.Minute,
				},
			},
		},
		{
			name: "已设置合法值保持不变",
			in: Config{
				Port:      1522,
				SlowSQLMs: 50,
				LogLevel:  LogLevelInfo,
				Pool:      storage.PoolConfig{MaxOpenConns: 8},
			},
			want: Config{
				Port:      1522,
				SlowSQLMs: 50,
				LogLevel:  LogLevelInfo,
				Pool: storage.PoolConfig{
					MaxOpenConns:    8,
					MaxIdleConns:    10,
					ConnMaxLifetime: time.Hour,
					ConnMaxIdleTime: 5 * time.Minute,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.in.WithDefaults()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConfigValidate(t *testing.T) {
	base := Config{User: "u", Host: "h", ServiceName: "svc", LogLevel: LogLevelWarn}
	tests := []struct {
		name    string
		mutate  func(Config) Config
		wantErr bool
		code    fverr.Code
	}{
		{name: "合法配置", mutate: func(c Config) Config { return c }},
		{name: "缺 User", mutate: func(c Config) Config { c.User = ""; return c }, wantErr: true, code: fverr.CodeValidation},
		{name: "缺 Host", mutate: func(c Config) Config { c.Host = ""; return c }, wantErr: true, code: fverr.CodeValidation},
		{name: "缺 ServiceName", mutate: func(c Config) Config { c.ServiceName = ""; return c }, wantErr: true, code: fverr.CodeValidation},
		{
			name:    "LogLevel 非法",
			mutate:  func(c Config) Config { c.LogLevel = LogLevel("loud"); return c },
			wantErr: true,
			code:    fverr.CodeValidation,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mutate(base).Validate()
			if !tt.wantErr {
				assert.NoError(t, err)
				return
			}
			assert.Error(t, err)
			if e, ok := fverr.From(err); ok {
				assert.Equal(t, tt.code, e.Code)
			} else {
				t.Fatalf("期望 *fverr.FvError, got %T", err)
			}
		})
	}
}

func TestConfigBuildDSN(t *testing.T) {
	tests := []struct {
		name string
		in   Config
		want string
	}{
		{
			name: "基础连接（Port 已默认）",
			in:   Config{User: "app", Password: "tiger", Host: "db", Port: 1521, ServiceName: "ORCL"},
			want: `user="app" password="tiger" connectString="db:1521/ORCL"`,
		},
		{
			name: "特殊字符全部转义",
			in:   Config{User: `a"b`, Password: `p@ss x`, Host: "db", Port: 1522, ServiceName: "S"},
			want: `user="a\"b" password="p@ss x" connectString="db:1522/S"`,
		},
		{
			name: "会话参数按键排序（值双层引号，兼容 godror 二次 logfmt 解码）",
			in: Config{
				User: "u", Password: "p", Host: "h", Port: 1521, ServiceName: "s",
				SessionParams: map[string]string{"TIME_ZONE": "UTC", "NLS_DATE_FORMAT": "YYYY-MM-DD"},
			},
			want: `user="u" password="p" connectString="h:1521/s" ` +
				`alterSession="NLS_DATE_FORMAT=\"YYYY-MM-DD\"" alterSession="TIME_ZONE=\"UTC\""`,
		},
		{
			name: "空会话参数无 alterSession 段",
			in:   Config{User: "u", Password: "p", Host: "h", Port: 1521, ServiceName: "s"},
			want: `user="u" password="p" connectString="h:1521/s"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.in.BuildDSN())
		})
	}
}

// TestBuildDSNGodrorRoundTrip 用 godror 官方 ParseDSN 验证 BuildDSN 可被完整还原，
// 尤其是含空格的 NLS 值不得被二次 logfmt 解码拆散（ORA-02248，真实库抓出）。
func TestBuildDSNGodrorRoundTrip(t *testing.T) {
	cfg := Config{
		User: "app", Password: `p@ss "x"`, Host: "db", Port: 1521, ServiceName: "ORCL",
		SessionParams: map[string]string{
			"NLS_DATE_FORMAT": "YYYY-MM-DD HH24:MI:SS",
			"TIME_ZONE":       "UTC",
		},
	}

	params, err := godror.ParseDSN(cfg.BuildDSN())
	require.NoError(t, err)
	assert.Equal(t, "app", params.Username)
	assert.Equal(t, `p@ss "x"`, params.Password.Secret())
	assert.Equal(t, "db:1521/ORCL", params.ConnectString)

	got := make(map[string]string, len(params.AlterSession))
	for _, kv := range params.AlterSession {
		got[kv[0]] = kv[1]
	}
	assert.Equal(t, cfg.SessionParams, got,
		"含空格的会话参数值必须原样还原为单条键值对")
}
