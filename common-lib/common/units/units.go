package units

import (
	"fmt"
	"strconv"
	"time"
)

const (
	// LayoutDate 是日期输入的约定格式（yyyyMMdd，如 20261021）。
	LayoutDate = "20060102"
	// LayoutDateTime 是日期时间输入的约定格式（yyyyMMddHHmmss，如
	// 20261021100000，与请求头 txTime 同一格式）。
	LayoutDateTime = "20060102150405"
)

// Bool 按 wire 约定转换布尔：输入为 int，1 → true，其余（含 0）→ false。
// int 零值天然表达「缺失即默认 false」。
func Bool(i int) bool { return i == 1 }

// ParseDate 解析日期字符串（yyyyMMdd，如 20261021）为当日 00:00:00 的
// 本地时间；格式非法返回错误。
func ParseDate(s string) (time.Time, error) {
	t, err := time.ParseInLocation(LayoutDate, s, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("units: invalid date %q, want yyyyMMdd", s)
	}
	return t, nil
}

// ParseDateTime 解析日期时间字符串（yyyyMMddHHmmss，如 20261021100000）
// 为本地时间；格式非法返回错误。
func ParseDateTime(s string) (time.Time, error) {
	t, err := time.ParseInLocation(LayoutDateTime, s, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("units: invalid datetime %q, want yyyyMMddHHmmss", s)
	}
	return t, nil
}

// ParseEpochMillis 解析毫秒时间戳字符串（13 位 Unix 毫秒，如
// 1761012000000）为本地时间；非 13 位或非数字报错。
func ParseEpochMillis(s string) (time.Time, error) {
	if len(s) != 13 {
		return time.Time{}, fmt.Errorf("units: invalid epoch millis %q, want 13 digits", s)
	}
	ms, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("units: invalid epoch millis %q, want 13 digits", s)
	}
	return time.UnixMilli(ms), nil
}
