package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	// ErrValueEmpty 表示 ValidateIsEmpty 发现值为空（nil、空串或 key 不存在）。
	ErrValueEmpty = errors.New("配置值为空")
	// ErrInvalidNumberValue 表示 ValidateIsNumber 发现值不是数字。
	ErrInvalidNumberValue = errors.New("配置值不是数字")
	// ErrInvalidBoolValue 表示 ValidateIsBoolean 发现值不是布尔值。
	ErrInvalidBoolValue = errors.New("配置值不是布尔值")
)

// ValidateIsEmpty 断言点分路径（如 "log.file.filePath"）的值非空：
// nil、空字符串或 key 不存在均视为空，返回 ErrValueEmpty。
func ValidateIsEmpty(m map[string]any, key string) error {
	v, ok := lookupValue(m, key)
	if !ok || v == nil {
		return fmt.Errorf("%w: %s", ErrValueEmpty, key)
	}
	if s, isStr := v.(string); isStr && s == "" {
		return fmt.Errorf("%w: %s", ErrValueEmpty, key)
	}
	return nil
}

// ValidateIsNumber 断言点分路径的值是数字（整型或浮点型任意宽度）。
// 与绑定的宽松规则一致：字符串值能解析为数字同样通过（如环境变量覆盖
// 产生的 "100"），否则返回 ErrInvalidNumberValue。
func ValidateIsNumber(m map[string]any, key string) error {
	v, ok := lookupValue(m, key)
	if !ok || !isNumber(v) {
		return fmt.Errorf("%w: %s", ErrInvalidNumberValue, key)
	}
	return nil
}

// ValidateIsBoolean 断言点分路径的值是布尔。与绑定的宽松规则一致：
// 字符串 "true"/"false"（不区分大小写）同样通过，否则返回
// ErrInvalidBoolValue。
func ValidateIsBoolean(m map[string]any, key string) error {
	v, ok := lookupValue(m, key)
	if !ok || !isBoolean(v) {
		return fmt.Errorf("%w: %s", ErrInvalidBoolValue, key)
	}
	return nil
}

// lookupValue 沿点分路径取叶子值：中途任一层 key 不存在或值不是
// map[string]any（最后一段除外）返回 (nil, false)。
func lookupValue(m map[string]any, path string) (any, bool) {
	i := strings.LastIndex(path, ".")
	if i < 0 {
		v, ok := m[path]
		return v, ok
	}
	parent := lookupMap(m, path[:i])
	if parent == nil {
		return nil, false
	}
	v, ok := parent[path[i+1:]]
	return v, ok
}

// isNumber 按 ValidateIsNumber 的宽松规则判断数字：数值类型直接通过，
// 字符串以 strconv.ParseFloat 结果为准。
func isNumber(v any) bool {
	switch t := v.(type) {
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return true
	case string:
		_, err := strconv.ParseFloat(t, 64)
		return err == nil
	default:
		return false
	}
}

// isBoolean 按 ValidateIsBoolean 的宽松规则判断布尔：bool 直接通过，
// 字符串仅 "true"/"false"（不区分大小写）通过。
func isBoolean(v any) bool {
	switch t := v.(type) {
	case bool:
		return true
	case string:
		switch strings.ToLower(t) {
		case "true", "false":
			return true
		}
	}
	return false
}
