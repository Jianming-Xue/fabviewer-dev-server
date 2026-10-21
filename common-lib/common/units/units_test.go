package units_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/common/units"
)

func TestBool(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want bool
	}{
		{"1 为 true", 1, true},
		{"0 为 false", 0, false},
		{"非 1 一律 false", 2, false},
		{"负数一律 false", -1, false},
		{"零值默认 false", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, units.Bool(tt.in))
		})
	}
}

func TestParseDate(t *testing.T) {
	t.Run("合法日期为当日零点本地时间", func(t *testing.T) {
		got, err := units.ParseDate("20261021")
		require.NoError(t, err)
		assert.Equal(t, time.Date(2026, 10, 21, 0, 0, 0, 0, time.Local), got)
	})

	t.Run("非法格式报错", func(t *testing.T) {
		for _, s := range []string{"", "2026-10-21", "2026102", "202610211", "20261021100000", "abcd"} {
			_, err := units.ParseDate(s)
			assert.Error(t, err, "输入 %q 应报错", s)
		}
	})
}

func TestParseDateTime(t *testing.T) {
	t.Run("合法日期时间为本地时间", func(t *testing.T) {
		got, err := units.ParseDateTime("20261021100000")
		require.NoError(t, err)
		assert.Equal(t, time.Date(2026, 10, 21, 10, 0, 0, 0, time.Local), got)
	})

	t.Run("与日期格式互不通用", func(t *testing.T) {
		_, err := units.ParseDateTime("20261021")
		assert.Error(t, err, "8 位日期串不是合法的日期时间")
	})

	t.Run("非法格式报错", func(t *testing.T) {
		for _, s := range []string{"", "2026-10-21 10:00:00", "202610211000", "20261021100000x"} {
			_, err := units.ParseDateTime(s)
			assert.Error(t, err, "输入 %q 应报错", s)
		}
	})
}

func TestParseEpochMillis(t *testing.T) {
	t.Run("13 位毫秒时间戳转本地时间", func(t *testing.T) {
		got, err := units.ParseEpochMillis("1761012000000")
		require.NoError(t, err)
		assert.Equal(t, time.UnixMilli(1761012000000), got)
	})

	t.Run("位数不对报错", func(t *testing.T) {
		for _, s := range []string{"", "176101200000", "17610120000000"} {
			_, err := units.ParseEpochMillis(s)
			assert.Error(t, err, "输入 %q 应报错", s)
		}
	})

	t.Run("非数字报错", func(t *testing.T) {
		_, err := units.ParseEpochMillis("176101200000x")
		assert.Error(t, err)
	})
}
