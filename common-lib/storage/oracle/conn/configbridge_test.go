package conn

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/storage/oracle/config"
)

// TestPrepareStmtEnabled 验证 PrepareStmt nil 视为启用的内联规则。
func TestPrepareStmtEnabled(t *testing.T) {
	enabled := true
	disabled := false
	tests := []struct {
		name string
		v    *bool
		want bool
	}{
		{"nil 视为启用", nil, true},
		{"显式启用", &enabled, true},
		{"显式关闭", &disabled, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, prepareStmtEnabled(tt.v))
		})
	}
	require.True(t, prepareStmtEnabled(nil))
}

func TestLogLevelToGorm(t *testing.T) {
	tests := []struct {
		in   config.LogLevel
		want int
	}{
		{config.LogLevelSilent, 1},
		{config.LogLevelError, 2},
		{config.LogLevelWarn, 3},
		{config.LogLevelInfo, 4},
		{config.LogLevel(""), 3},
		{config.LogLevel("loud"), 3},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, int(logLevelToGorm(tt.in)), "level=%q", tt.in)
	}
}
