package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeConfig 在临时目录写配置文件并返回路径。
func writeConfig(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

const sampleYAML = `
log:
  level: info
  file:
    path: systemlogs/app.log
    maxSizeMB: 100
    insecure: true
storage:
  oracle:
    user: app
    sessionParams:
      nlsDateFormat: yyyyMMdd
`

func TestLoadConfigYAML(t *testing.T) {
	path := writeConfig(t, "application.yml", sampleYAML)
	t.Setenv("testapp_CFG", path)

	m, err := LoadConfig("testapp")
	require.NoError(t, err)

	file, ok := m["log"].(map[string]any)["file"].(map[string]any)
	require.True(t, ok, "嵌套两层应取到子 map")
	assert.Equal(t, "systemlogs/app.log", file["path"])
	assert.Equal(t, 100, file["maxSizeMB"]) // yaml 数值为 int
	assert.Equal(t, true, file["insecure"])
	assert.Equal(t, "info", m["log"].(map[string]any)["level"])

	oracle := m["storage"].(map[string]any)["oracle"].(map[string]any)
	assert.Equal(t, map[string]any{"nlsDateFormat": "yyyyMMdd"}, oracle["sessionParams"])
}

func TestLoadConfigJSON(t *testing.T) {
	path := writeConfig(t, "application.json", `{"log":{"level":"debug"}}`)
	t.Setenv("testapp_CFG", path)

	m, err := LoadConfig("testapp")
	require.NoError(t, err)
	assert.Equal(t, "debug", m["log"].(map[string]any)["level"])
}

func TestLoadConfigInvalidExt(t *testing.T) {
	path := writeConfig(t, "application.toml", "x = 1")
	t.Setenv("testapp_CFG", path)

	_, err := LoadConfig("testapp")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidConfigFile)
}

func TestLoadConfigDefaultPath(t *testing.T) {
	// 未设 CFG 环境变量时取工作目录下的 application.yml
	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "application.yml"), []byte("a: 1\n"), 0o600))

	m, err := LoadConfig("testapp")
	require.NoError(t, err)
	assert.Equal(t, 1, m["a"])
}

func TestLoadConfigFileMissing(t *testing.T) {
	t.Setenv("testapp_CFG", filepath.Join(t.TempDir(), "nope.yml"))

	_, err := LoadConfig("testapp")
	require.Error(t, err)
	var pathErr *os.PathError
	assert.ErrorAs(t, err, &pathErr)
}

func TestLoadConfigEmptyFile(t *testing.T) {
	path := writeConfig(t, "application.yml", "")
	t.Setenv("testapp_CFG", path)

	m, err := LoadConfig("testapp")
	require.NoError(t, err)
	assert.Empty(t, m)
}

func TestLoadConfigParseError(t *testing.T) {
	path := writeConfig(t, "application.yml", "log: [unclosed")
	t.Setenv("testapp_CFG", path)

	_, err := LoadConfig("testapp")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrInvalidConfigFile, "解析失败与非法后缀是不同错误")
}

func TestApplyEnvOverrides(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		env     map[string]string
		want    map[string]any
		wantNot string // 断言保留原值的路径叶子 key
	}{
		{
			name: "叶子覆盖",
			yaml: "log:\n  level: info\n",
			env:  map[string]string{"testapp.log.level": "debug"},
			want: map[string]any{"log": map[string]any{"level": "debug"}},
		},
		{
			name: "深层叶子覆盖且类型变字符串",
			yaml: "log:\n  file:\n    maxSizeMB: 100\n",
			env:  map[string]string{"testapp.log.file.maxSizeMB": "200"},
			want: map[string]any{"log": map[string]any{"file": map[string]any{"maxSizeMB": "200"}}},
		},
		{
			name:    "未命中保留原值",
			yaml:    "log:\n  level: info\n",
			env:     map[string]string{"testapp.other.key": "x"},
			want:    map[string]any{"log": map[string]any{"level": "info"}},
			wantNot: "testapp.log.level",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			path := writeConfig(t, "application.yml", tt.yaml)
			t.Setenv("testapp_CFG", path)

			m, err := LoadConfig("testapp")
			require.NoError(t, err)
			assert.Equal(t, tt.want, m)
			if tt.wantNot != "" {
				_, hit := os.LookupEnv(tt.wantNot)
				assert.False(t, hit, "未设置的环境变量不应覆盖")
			}
		})
	}
}

func TestGet(t *testing.T) {
	m := map[string]any{
		"log": map[string]any{
			"level": "info",
			"file":  map[string]any{"path": "app.log"},
		},
	}

	assert.Equal(t, m["log"], Get(m, "log"))
	assert.Equal(t, m["log"].(map[string]any)["file"], Get(m, "log.file"))
	assert.Nil(t, Get(m, "log.level"), "叶子是标量不是 map")
	assert.Nil(t, Get(m, "log.level.deeper"), "中途断层")
	assert.Nil(t, Get(m, "missing"), "未命中")
}

// fileSection 是 GetSection 绑定的测试目标：混合无 tag 字段、tag 字段与
// 嵌套结构。
type fileSection struct {
	Path       string
	MaxSizeMB  int
	Insecure   bool
	MaxBackups int `mapstructure:"maxBackups"`
	Pool       struct {
		MaxOpen int
	}
}

func TestGetSection(t *testing.T) {
	m := map[string]any{
		"log": map[string]any{
			"file": map[string]any{
				"path":       "systemlogs/app.log",
				"maxSizeMB":  100,
				"insecure":   true,
				"maxBackups": 200,
				"pool":       map[string]any{"maxOpen": 30},
			},
		},
	}

	got, err := GetSection[fileSection](m, "log.file")
	require.NoError(t, err)
	assert.Equal(t, "systemlogs/app.log", got.Path)
	assert.Equal(t, 100, got.MaxSizeMB)
	assert.True(t, got.Insecure)
	assert.Equal(t, 200, got.MaxBackups, "tag 命中")
	assert.Equal(t, 30, got.Pool.MaxOpen, "嵌套结构继续下钻绑定")
}

func TestGetSectionCaseInsensitive(t *testing.T) {
	// yaml key 小写开头，无 tag 字段按完整字段名忽略大小写匹配
	m := map[string]any{"file": map[string]any{"maxsizemb": 50, "PATH": "a.log"}}

	got, err := GetSection[fileSection](m, "file")
	require.NoError(t, err)
	assert.Equal(t, 50, got.MaxSizeMB)
	assert.Equal(t, "a.log", got.Path)
}

func TestGetSectionWeaklyTyped(t *testing.T) {
	// 环境变量覆盖场景：map 中已是字符串，绑定按目标字段类型转换
	m := map[string]any{"file": map[string]any{"maxSizeMB": "300", "insecure": "true"}}

	got, err := GetSection[fileSection](m, "file")
	require.NoError(t, err)
	assert.Equal(t, 300, got.MaxSizeMB)
	assert.True(t, got.Insecure)
}

func TestGetSectionNotFound(t *testing.T) {
	m := map[string]any{"log": map[string]any{"level": "info"}}

	_, err := GetSection[fileSection](m, "log.file")
	require.ErrorIs(t, err, ErrSectionNotFound)

	_, err = GetSection[fileSection](m, "missing")
	require.ErrorIs(t, err, ErrSectionNotFound)

	_, err = GetSection[fileSection](m, "")
	require.ErrorIs(t, err, ErrSectionNotFound, "空路径必然未命中")
}

func TestGetSectionDecodeError(t *testing.T) {
	m := map[string]any{"file": map[string]any{"maxSizeMB": "not-a-number"}}

	_, err := GetSection[fileSection](m, "file")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrSectionNotFound, "绑定失败与段不存在是不同错误")
}

func TestValidateIsEmpty(t *testing.T) {
	m := map[string]any{
		"s":   "value",
		"emp": "",
		"n":   nil,
		"i":   0,
		"sub": map[string]any{"deep": "x"},
	}

	assert.NoError(t, ValidateIsEmpty(m, "s"))
	assert.NoError(t, ValidateIsEmpty(m, "i"), "数字零值不算空")
	assert.NoError(t, ValidateIsEmpty(m, "sub.deep"))

	assert.ErrorIs(t, ValidateIsEmpty(m, "emp"), ErrValueEmpty)
	assert.ErrorIs(t, ValidateIsEmpty(m, "n"), ErrValueEmpty)
	assert.ErrorIs(t, ValidateIsEmpty(m, "missing"), ErrValueEmpty, "key 不存在视为空")
	assert.ErrorIs(t, ValidateIsEmpty(m, "sub.missing"), ErrValueEmpty)
	assert.ErrorIs(t, ValidateIsEmpty(m, "s.deeper"), ErrValueEmpty, "中途断层视为空")
}

func TestValidateIsNumber(t *testing.T) {
	m := map[string]any{
		"i":    100,
		"neg":  -3,
		"i64":  int64(9),
		"u":    uint(7),
		"f":    3.14,
		"sf":   float32(1.5),
		"si":   "200", // 环境变量覆盖场景
		"sflt": "3.14",
		"bad":  "abc",
		"b":    true,
		"sub":  map[string]any{"n": 5},
	}

	assert.NoError(t, ValidateIsNumber(m, "i"))
	assert.NoError(t, ValidateIsNumber(m, "neg"))
	assert.NoError(t, ValidateIsNumber(m, "i64"))
	assert.NoError(t, ValidateIsNumber(m, "u"))
	assert.NoError(t, ValidateIsNumber(m, "f"))
	assert.NoError(t, ValidateIsNumber(m, "sf"))
	assert.NoError(t, ValidateIsNumber(m, "si"))
	assert.NoError(t, ValidateIsNumber(m, "sflt"))
	assert.NoError(t, ValidateIsNumber(m, "sub.n"))

	assert.ErrorIs(t, ValidateIsNumber(m, "bad"), ErrInvalidNumberValue)
	assert.ErrorIs(t, ValidateIsNumber(m, "b"), ErrInvalidNumberValue)
	assert.ErrorIs(t, ValidateIsNumber(m, "missing"), ErrInvalidNumberValue, "key 不存在视为非数字")
}

func TestValidateIsBoolean(t *testing.T) {
	m := map[string]any{
		"t":     true,
		"fl":    false,
		"st":    "true",  // 环境变量覆盖场景
		"sF":    "FALSE", // 大小写不敏感
		"sf":    "false",
		"bad":   "yes",
		"i":     1,
		"empty": "",
	}

	assert.NoError(t, ValidateIsBoolean(m, "t"))
	assert.NoError(t, ValidateIsBoolean(m, "fl"))
	assert.NoError(t, ValidateIsBoolean(m, "st"))
	assert.NoError(t, ValidateIsBoolean(m, "sF"))
	assert.NoError(t, ValidateIsBoolean(m, "sf"))

	assert.ErrorIs(t, ValidateIsBoolean(m, "bad"), ErrInvalidBoolValue)
	assert.ErrorIs(t, ValidateIsBoolean(m, "i"), ErrInvalidBoolValue)
	assert.ErrorIs(t, ValidateIsBoolean(m, "empty"), ErrInvalidBoolValue)
	assert.ErrorIs(t, ValidateIsBoolean(m, "missing"), ErrInvalidBoolValue, "key 不存在视为非布尔")
}

func TestLookupValue(t *testing.T) {
	m := map[string]any{
		"a": map[string]any{"b": map[string]any{"c": "leaf"}},
		"x": "scalar",
	}

	v, ok := lookupValue(m, "a.b.c")
	assert.True(t, ok)
	assert.Equal(t, "leaf", v)

	v, ok = lookupValue(m, "x")
	assert.True(t, ok)
	assert.Equal(t, "scalar", v)

	_, ok = lookupValue(m, "a.missing")
	assert.False(t, ok)

	_, ok = lookupValue(m, "x.deeper")
	assert.False(t, ok, "标量下继续下钻视为不存在")
}

func TestBind(t *testing.T) {
	type durations struct {
		Addr    string        `mapstructure:"addr"`
		Timeout time.Duration `mapstructure:"timeout"`
		Level   string        `mapstructure:"level"`
	}

	t.Run("保留目标已有值、map 中的键覆盖", func(t *testing.T) {
		out := durations{Addr: ":8080", Timeout: 30 * time.Second, Level: "info"}
		err := Bind(map[string]any{"addr": ":9090", "level": "debug"}, &out)
		require.NoError(t, err)
		assert.Equal(t, ":9090", out.Addr, "出现的键覆盖默认值")
		assert.Equal(t, 30*time.Second, out.Timeout, "未出现的键保留默认值")
		assert.Equal(t, "debug", out.Level)
	})

	t.Run("字符串时长转 Duration", func(t *testing.T) {
		out := durations{}
		err := Bind(map[string]any{"timeout": "5s"}, &out)
		require.NoError(t, err)
		assert.Equal(t, 5*time.Second, out.Timeout)
	})

	t.Run("弱类型转换", func(t *testing.T) {
		out := struct {
			N int `mapstructure:"n"`
		}{}
		err := Bind(map[string]any{"n": "200"}, &out)
		require.NoError(t, err)
		assert.Equal(t, 200, out.N)
	})

	t.Run("无法转换时报错", func(t *testing.T) {
		out := durations{}
		err := Bind(map[string]any{"timeout": "abc"}, &out)
		assert.Error(t, err)
	})
}
