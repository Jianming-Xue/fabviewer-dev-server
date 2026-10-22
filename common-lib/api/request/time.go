package request

import (
	"context"
	"strings"
	"time"

	"github.com/fab-viewer/common-lib/common/commonerrors"
	"github.com/fab-viewer/common-lib/common/units"
)

// Time 校验并解析时间字符串参数，按内容长度识别 wire 格式：8 位
// yyyyMMdd（当日零点）、14 位 yyyyMMddHHmmss、13 位毫秒时间戳。
// 空串/纯空白返回零值不报错（可选参数语义）；非法格式返回公共错误
// InvalidTime（commonerrors 字典按请求语言翻译，errMsg 携带参数名与原值）。
func Time(ctx context.Context, name, v string) (time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}, nil
	}
	t, err := parseTimeByLength(v)
	if err != nil {
		return time.Time{}, commonerrors.NewInvalidTime(ctx).InsertArgs(name, v)
	}
	return t, nil
}

// parseTimeByLength 按字符串长度选择解析格式，长度不识别即报错。
func parseTimeByLength(s string) (time.Time, error) {
	switch len(s) {
	case len("20261021"): // 8 位日期
		return units.ParseDate(s)
	case len("1761012000000"): // 13 位毫秒时间戳
		return units.ParseEpochMillis(s)
	case len("20261021100000"): // 14 位日期时间
		return units.ParseDateTime(s)
	}
	return time.Time{}, errBadTime
}
