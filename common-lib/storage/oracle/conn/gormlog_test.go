package conn

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	gormlogger "gorm.io/gorm/logger"

	"github.com/fab-viewer/common-lib/storage/oracle/config"
)

func TestGormLoggerTrace(t *testing.T) {
	genericErr := errors.New("ORA-00942")
	tests := []struct {
		name       string
		level      config.LogLevel
		slow       time.Duration
		elapsed    time.Duration
		err        error
		wantLevels []string
		wantMsg    string // 命中日志的 msg（取首条）
	}{
		{
			name: "silent 级别全静默", level: config.LogLevelSilent, slow: 200 * time.Millisecond,
			elapsed: time.Second, err: genericErr, wantLevels: nil,
		},
		{
			name: "error 级别输出错误 SQL", level: config.LogLevelError, slow: 200 * time.Millisecond,
			elapsed: time.Millisecond, err: genericErr, wantLevels: []string{"error"}, wantMsg: "gorm sql error",
		},
		{
			name: "error 级别跳过 ErrRecordNotFound", level: config.LogLevelError, slow: 200 * time.Millisecond,
			elapsed: time.Millisecond, err: gormlogger.ErrRecordNotFound, wantLevels: nil,
		},
		{
			name: "warn 级别不输出普通 SQL", level: config.LogLevelWarn, slow: 200 * time.Millisecond,
			elapsed: time.Millisecond, err: nil, wantLevels: nil,
		},
		{
			name: "warn 级别输出慢 SQL", level: config.LogLevelWarn, slow: 200 * time.Millisecond,
			elapsed: 300 * time.Millisecond, err: nil, wantLevels: []string{"warn"}, wantMsg: "gorm slow sql",
		},
		{
			name: "阈值未配置不判定慢 SQL", level: config.LogLevelWarn, slow: 0,
			elapsed: time.Hour, err: nil, wantLevels: nil,
		},
		{
			name: "error 级别错误优先于慢 SQL", level: config.LogLevelError, slow: 200 * time.Millisecond,
			elapsed: time.Second, err: genericErr, wantLevels: []string{"error"}, wantMsg: "gorm sql error",
		},
		{
			name: "info 级别输出普通 SQL", level: config.LogLevelInfo, slow: 200 * time.Millisecond,
			elapsed: time.Millisecond, err: nil, wantLevels: []string{"info"}, wantMsg: "gorm sql",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &recLogger{}
			l := newGormLogger(rec, tt.level, tt.slow)
			l.Trace(context.Background(),
				time.Now().Add(-tt.elapsed),
				func() (string, int64) { return "SELECT 1 FROM DUAL", 3 },
				tt.err,
			)
			assert.Equal(t, tt.wantLevels, rec.levels())
			if tt.wantMsg != "" {
				assert.Equal(t, tt.wantMsg, rec.entries[0].msg)
			}
		})
	}
}

func TestGormLoggerTraceRowsUnknown(t *testing.T) {
	rec := &recLogger{}
	l := newGormLogger(rec, config.LogLevelInfo, 200*time.Millisecond)
	l.Trace(context.Background(), time.Now(),
		func() (string, int64) { return "DELETE FROM t", -1 }, nil)
	assert.Equal(t, []string{"info"}, rec.levels())
}

func TestGormLoggerInfoWarnError(t *testing.T) {
	tests := []struct {
		name       string
		level      config.LogLevel
		call       func(l *gormLogger)
		wantLevels []string
		wantMsg    string
	}{
		{
			name: "info 级别输出 Info 消息（含格式化）", level: config.LogLevelInfo,
			call:       func(l *gormLogger) { l.Info(context.Background(), "dialect %s", "oracle") },
			wantLevels: []string{"info"}, wantMsg: "dialect oracle",
		},
		{
			name: "warn 级别不输出 Info", level: config.LogLevelWarn,
			call:       func(l *gormLogger) { l.Info(context.Background(), "m") },
			wantLevels: nil,
		},
		{
			name: "warn 级别输出 Warn 消息", level: config.LogLevelWarn,
			call:       func(l *gormLogger) { l.Warn(context.Background(), "w %d", 7) },
			wantLevels: []string{"warn"}, wantMsg: "w 7",
		},
		{
			name: "error 级别输出 Error 消息", level: config.LogLevelError,
			call:       func(l *gormLogger) { l.Error(context.Background(), "e") },
			wantLevels: []string{"error"}, wantMsg: "e",
		},
		{
			name: "silent 级别不输出 Error", level: config.LogLevelSilent,
			call:       func(l *gormLogger) { l.Error(context.Background(), "e") },
			wantLevels: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &recLogger{}
			l := newGormLogger(rec, tt.level, time.Millisecond)
			tt.call(l)
			assert.Equal(t, tt.wantLevels, rec.levels())
			if tt.wantMsg != "" {
				assert.Equal(t, tt.wantMsg, rec.entries[0].msg)
			}
		})
	}
}

func TestGormLoggerLogMode(t *testing.T) {
	rec := &recLogger{}
	l := newGormLogger(rec, config.LogLevelInfo, time.Millisecond)

	quiet := l.LogMode(gormlogger.Silent)
	quiet.Info(context.Background(), "should be silent")
	assert.Empty(t, rec.entries, "派生 silent 实例不应输出")

	l.Info(context.Background(), "original still info")
	assert.Equal(t, []string{"info"}, rec.levels(), "原实例级别不受 LogMode 影响")

	var _ gormlogger.Interface = l
}

func TestNewGormLoggerNilFallback(t *testing.T) {
	l := newGormLogger(nil, config.LogLevelWarn, time.Millisecond)
	assert.NotNil(t, l)
	// logx.Default() 初始为 Nop，不 panic 即通过
	l.Error(context.Background(), "nop sink")
}
