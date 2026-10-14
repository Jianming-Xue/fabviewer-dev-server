package oracle

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/fab-viewer/common-lib/errs"
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
		code    errs.Code
	}{
		{name: "合法配置", mutate: func(c Config) Config { return c }},
		{name: "缺 User", mutate: func(c Config) Config { c.User = ""; return c }, wantErr: true, code: errs.CodeValidation},
		{name: "缺 Host", mutate: func(c Config) Config { c.Host = ""; return c }, wantErr: true, code: errs.CodeValidation},
		{name: "缺 ServiceName", mutate: func(c Config) Config { c.ServiceName = ""; return c }, wantErr: true, code: errs.CodeValidation},
		{
			name:    "LogLevel 非法",
			mutate:  func(c Config) Config { c.LogLevel = LogLevel("loud"); return c },
			wantErr: true,
			code:    errs.CodeValidation,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mutate(base).validate()
			if !tt.wantErr {
				assert.NoError(t, err)
				return
			}
			assert.Error(t, err)
			if e, ok := errs.From(err); ok {
				assert.Equal(t, tt.code, e.Code)
			} else {
				t.Fatalf("期望 *errs.Error, got %T", err)
			}
		})
	}
}

func TestConfigPrepareStmtEnabled(t *testing.T) {
	falseVal := false
	trueVal := true
	tests := []struct {
		name string
		in   *bool
		want bool
	}{
		{name: "nil 默认启用", in: nil, want: true},
		{name: "显式 true", in: &trueVal, want: true},
		{name: "显式 false 关闭", in: &falseVal, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, (Config{PrepareStmt: tt.in}).prepareStmtEnabled())
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
			name: "会话参数按键排序",
			in: Config{
				User: "u", Password: "p", Host: "h", Port: 1521, ServiceName: "s",
				SessionParams: map[string]string{"TIME_ZONE": "UTC", "NLS_DATE_FORMAT": "YYYY-MM-DD"},
			},
			want: `user="u" password="p" connectString="h:1521/s" ` +
				`alterSession="NLS_DATE_FORMAT=YYYY-MM-DD" alterSession="TIME_ZONE=UTC"`,
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

func TestLogLevelToGorm(t *testing.T) {
	tests := []struct {
		in   LogLevel
		want int
	}{
		{LogLevelSilent, 1},
		{LogLevelError, 2},
		{LogLevelWarn, 3},
		{LogLevelInfo, 4},
		{LogLevel(""), 3},
		{LogLevel("loud"), 3},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, int(tt.in.toGorm()), "level=%q", tt.in)
	}
}
