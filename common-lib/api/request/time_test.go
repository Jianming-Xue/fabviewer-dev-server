package request_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/api/request"
	"github.com/fab-viewer/common-lib/common/bizerror"
	"github.com/fab-viewer/common-lib/common/commoncodes"
	"github.com/fab-viewer/common-lib/common/rctx"
)

func TestTime(t *testing.T) {
	t.Run("按长度识别三种 wire 格式", func(t *testing.T) {
		d, err := request.Time(context.Background(), "createdFrom", "20261021")
		require.NoError(t, err)
		assert.Equal(t, time.Date(2026, 10, 21, 0, 0, 0, 0, time.Local), d, "8 位日期")

		dt, err := request.Time(context.Background(), "createdAt", "20261021100000")
		require.NoError(t, err)
		assert.Equal(t, time.Date(2026, 10, 21, 10, 0, 0, 0, time.Local), dt, "14 位日期时间")

		ms, err := request.Time(context.Background(), "ts", "1761012000000")
		require.NoError(t, err)
		assert.Equal(t, time.UnixMilli(1761012000000), ms, "13 位毫秒")
	})

	t.Run("空串与纯空白按可选参数返回零值", func(t *testing.T) {
		for _, v := range []string{"", "   "} {
			got, err := request.Time(context.Background(), "createdFrom", v)
			require.NoError(t, err, "输入 %q 不应报错", v)
			assert.True(t, got.IsZero())
		}
	})

	t.Run("去首尾空白后解析", func(t *testing.T) {
		got, err := request.Time(context.Background(), "d", " 20261021 ")
		require.NoError(t, err)
		assert.Equal(t, time.Date(2026, 10, 21, 0, 0, 0, 0, time.Local), got)
	})

	t.Run("非法格式报公共错误 InvalidTime（默认英文）", func(t *testing.T) {
		for _, v := range []string{"2026-10-21", "2026102", "202610211", "12345678901", "20261x2110000"} {
			_, err := request.Time(context.Background(), "createdFrom", v)
			require.Error(t, err, "输入 %q 应报错", v)
			e, ok := bizerror.From(err)
			require.True(t, ok)
			assert.Equal(t, commoncodes.InvalidTime, e.Code)
			assert.Contains(t, e.Message, "createdFrom", "errMsg 携带参数名")
			assert.Contains(t, e.Message, v, "errMsg 携带原值")
		}
	})

	t.Run("请求语言为中文时 errMsg 为中文", func(t *testing.T) {
		ctx := rctx.SetRuntimeContext(context.Background(), rctx.RuntimeContext{}.WithLang("zh"))
		_, err := request.Time(ctx, "createdFrom", "bad")
		e, ok := bizerror.From(err)
		require.True(t, ok)
		assert.Contains(t, e.Message, "参数 createdFrom")
	})
}

func TestWrapTimeString(t *testing.T) {
	// queryDTO 模拟入参对象：时间字段是 wire 字符串；rangeVO 模拟 service 层
	// VO：同名字段是 time.Time。
	type queryDTO struct {
		From  string // 8 位日期
		At    string // 14 位日期时间
		Ts    string // 13 位毫秒
		From2 string
	}
	type rangeVO struct {
		From time.Time
		At   time.Time
		Ts   time.Time
	}

	t.Run("字符串自动解析为 time.Time", func(t *testing.T) {
		vo, err := request.Wrap[rangeVO](queryDTO{
			From: "20261021",
			At:   "20261021100000",
			Ts:   "1761012000000",
		})
		require.NoError(t, err)
		assert.Equal(t, time.Date(2026, 10, 21, 0, 0, 0, 0, time.Local), vo.From)
		assert.Equal(t, time.Date(2026, 10, 21, 10, 0, 0, 0, time.Local), vo.At)
		assert.Equal(t, time.UnixMilli(1761012000000), vo.Ts)
	})

	t.Run("空串保持零值", func(t *testing.T) {
		vo, err := request.Wrap[rangeVO](queryDTO{})
		require.NoError(t, err)
		assert.True(t, vo.From.IsZero(), "可选时间参数缺省为零值")
	})

	t.Run("非法格式报错", func(t *testing.T) {
		_, err := request.Wrap[rangeVO](queryDTO{At: "2026-10-21 10:00:00"})
		assert.Error(t, err, "非约定格式不得静默通过")
	})
}
