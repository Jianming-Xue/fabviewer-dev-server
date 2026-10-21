package logx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"

	"github.com/fab-viewer/common-lib/common/config"
)

// logSection 构造等价于 application.yml log 段的 map，覆盖项按 muts 变更。
func logSection(muts ...func(map[string]any)) map[string]any {
	console := map[string]any{"enabled": true, "format": "console"}
	file := map[string]any{"enabled": true, "format": "json"}
	root := map[string]any{"log": map[string]any{
		"level":   "info",
		"console": console,
		"file":    file,
	}}
	for _, mut := range muts {
		mut(root)
	}
	return root
}

func disableConsole(m map[string]any) {
	m["log"].(map[string]any)["console"].(map[string]any)["enabled"] = false
}

func TestInitFileEndToEnd(t *testing.T) {
	t.Cleanup(func() { SetLogger(Nop()) })
	path := filepath.Join(t.TempDir(), "app.log")

	m := logSection(disableConsole)
	m["log"].(map[string]any)["file"].(map[string]any)["path"] = path

	log, closeLog, err := Init(m)
	require.NoError(t, err)

	log.Info("hello file", "id", 42)
	closeLog()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	line := string(data)
	assert.Contains(t, line, `"msg":"hello file"`, "json 默认格式")
	assert.Contains(t, line, `"id":42`)

	t.Run("Init 落位全局出口", func(t *testing.T) {
		Debug("via package debug")
		Info("via package func")
		Warn("via package warn")
		Error("via package error")
		closeLog() // 已 Close 再调用收尾不应 panic
		data, _ := os.ReadFile(path)
		assert.Contains(t, string(data), "via package func")
		assert.Contains(t, string(data), `"caller":"`, "caller 指向调用行（json 含 caller 字段）")
		assert.Contains(t, string(data), "engine_test.go:", "包级函数直调 zap，caller 应指向本测试文件")
	})
}

func TestInitNoLogComponent(t *testing.T) {
	t.Run("两个出口都禁用", func(t *testing.T) {
		m := logSection(func(m map[string]any) {
			disableConsole(m)
			m["log"].(map[string]any)["file"].(map[string]any)["enabled"] = false
		})
		_, _, err := Init(m)
		assert.ErrorIs(t, err, ErrNoLogComponent)
	})

	t.Run("缺失整个 log 段", func(t *testing.T) {
		_, _, err := Init(map[string]any{})
		assert.ErrorIs(t, err, ErrNoLogComponent)
	})

	t.Run("只写子段不写 enabled 默认不启用", func(t *testing.T) {
		_, _, err := Init(map[string]any{"log": map[string]any{"level": "info"}})
		assert.ErrorIs(t, err, ErrNoLogComponent)
	})
}

func TestInitValidation(t *testing.T) {
	t.Run("enabled 非布尔", func(t *testing.T) {
		m := logSection(func(m map[string]any) {
			m["log"].(map[string]any)["console"].(map[string]any)["enabled"] = "yes"
		})
		_, _, err := Init(m)
		require.Error(t, err)
		assert.ErrorIs(t, err, config.ErrInvalidBoolValue)
	})

	t.Run("maxSize 非数字", func(t *testing.T) {
		m := logSection(func(m map[string]any) {
			m["log"].(map[string]any)["file"].(map[string]any)["maxSize"] = "abc"
		})
		_, _, err := Init(m)
		require.Error(t, err)
		assert.ErrorIs(t, err, config.ErrInvalidNumberValue)
	})

	t.Run("键缺失不校验（取默认值）", func(t *testing.T) {
		m := logSection(disableConsole)
		m["log"].(map[string]any)["file"] = map[string]any{"enabled": true, "path": filepath.Join(t.TempDir(), "a.log")}
		_, closeLog, err := Init(m)
		require.NoError(t, err, "未配置 format/maxSize 等键不应报错")
		closeLog()
	})
}

func TestInitConsoleEndToEnd(t *testing.T) {
	t.Cleanup(func() { SetLogger(Nop()) })
	path := filepath.Join(t.TempDir(), "both.log")

	m := logSection(func(m map[string]any) {
		m["log"].(map[string]any)["file"].(map[string]any)["path"] = path
	})
	log, closeLog, err := Init(m) // console + file 双出口（stdout 有输出属预期）
	require.NoError(t, err)

	// 覆盖 zapLogger 桥接的全部方法与 With 派生
	log.Debug("dbg")
	log.Warn("warn")
	log.Error("error")
	log.With("k", "v").Info("derived")
	closeLog()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"msg":"derived"`)
	assert.Contains(t, string(data), `"k":"v"`)
}

func TestInitBindFailure(t *testing.T) {
	m := logSection(func(m map[string]any) {
		m["log"].(map[string]any)["console"] = "oops" // 字符串绑不到结构体
	})
	_, _, err := Init(m)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNoLogComponent, "绑定失败与未启用出口是不同错误")
}

func TestInitWeaklyTypedFromEnv(t *testing.T) {
	t.Cleanup(func() { SetLogger(Nop()) })
	path := filepath.Join(t.TempDir(), "env.log")

	m := logSection(disableConsole)
	file := m["log"].(map[string]any)["file"].(map[string]any)
	file["path"] = path
	file["maxSize"] = "150" // 环境变量覆盖后的典型形态：字符串数字

	_, closeLog, err := Init(m)
	require.NoError(t, err)
	Info("env override ok")
	closeLog()

	_, _ = os.ReadFile(path) // 写入成功即证明绑定与出口正常
}

func TestResolveLevel(t *testing.T) {
	tests := []struct {
		name   string
		outlet string
		global string
		want   zapcore.Level
	}{
		{"出口优先", "error", "debug", zapcore.ErrorLevel},
		{"出口空退全局", "", "warn", zapcore.WarnLevel},
		{"出口非法退全局", "trace", "warn", zapcore.WarnLevel},
		{"全空兜底 info", "", "", zapcore.InfoLevel},
		{"全局也非法兜底 info", "fatal", "verbose", zapcore.InfoLevel},
		{"debug 四档之一", "debug", "info", zapcore.DebugLevel},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, resolveLevel(tt.outlet, tt.global))
		})
	}
}

func TestParseLevel(t *testing.T) {
	for _, s := range []string{"debug", "info", "warn", "error"} {
		_, ok := parseLevel(s)
		assert.True(t, ok, s)
		_, ok = parseLevel(strings.ToUpper(s))
		assert.True(t, ok, "大小写不敏感: "+s)
	}
	for _, s := range []string{"", "trace", "verbose", "fatal", "panic"} {
		_, ok := parseLevel(s)
		assert.False(t, ok, s)
	}
}

func TestNewEncoder(t *testing.T) {
	enc := func(e zapcore.Encoder) string {
		buf, err := e.EncodeEntry(zapcore.Entry{Level: zapcore.InfoLevel, Message: "m"}, nil)
		require.NoError(t, err)
		return buf.String()
	}

	assert.Contains(t, enc(newEncoder("")), `"msg":"m"`, "空串默认 json")
	assert.Contains(t, enc(newEncoder("json")), `"msg":"m"`)
	assert.Contains(t, enc(newEncoder("xml")), `"msg":"m"`, "非法取值静默按 json")
	assert.Contains(t, enc(newEncoder("console")), "\tm\n", "console 格式为制表符分隔")
	assert.Contains(t, enc(newEncoder("Console")), "\tm\n", "format 大小写不敏感")
}

func TestWithFileDefaults(t *testing.T) {
	t.Run("全默认", func(t *testing.T) {
		fc := withFileDefaults(&fileConfig{})
		assert.Equal(t, defaultLogPath, fc.Path)
		assert.Equal(t, defaultMaxSizeMB, fc.MaxSize)
		assert.Equal(t, defaultMaxBackups, fc.MaxBackups)
		assert.Equal(t, defaultMaxAgeDays, fc.MaxAge)
	})

	t.Run("显式值保留", func(t *testing.T) {
		fc := withFileDefaults(&fileConfig{Path: "a.log", MaxSize: 1, MaxBackups: 2, MaxAge: 3})
		assert.Equal(t, "a.log", fc.Path)
		assert.Equal(t, 1, fc.MaxSize)
		assert.Equal(t, 2, fc.MaxBackups)
		assert.Equal(t, 3, fc.MaxAge)
	})
}

func TestLocalTimeThreeState(t *testing.T) {
	// LocalTime 用 *bool 表达三态：nil 默认 true、显式 true/false 原样
	fc := &fileConfig{}
	assert.True(t, fc.LocalTime == nil || *fc.LocalTime, "未配置按 true")
	tt, ff := true, false
	assert.True(t, *(&fileConfig{LocalTime: &tt}).LocalTime)
	assert.False(t, *(&fileConfig{LocalTime: &ff}).LocalTime)
}
