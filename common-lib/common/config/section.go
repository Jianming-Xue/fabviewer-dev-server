package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-viper/mapstructure/v2"
)

// ErrSectionNotFound 表示 GetSection 的点分路径在配置中不存在（或中途
// 断层：某一段的值不是子 map）。
var ErrSectionNotFound = errors.New("配置段不存在")

// Get 返回点分路径（如 "log.file"）对应的原始子 map，不做任何绑定。
// 路径未命中、中途断层或终点是标量值时返回 nil，不报错。
func Get(m map[string]any, path string) map[string]any {
	return lookupMap(m, path)
}

// GetSection 将点分路径（如 "log.file"）对应的子 map 绑定到 T 并返回。
// 字段匹配规则：mapstructure tag 优先，无 tag 字段按完整字段名忽略大小写
// 匹配（如 yml key "maxSize" 命中字段 MaxSize）。绑定规则同 Bind
// （WeaklyTypedInput + 字符串时长钩子）。路径未命中时返回 ErrSectionNotFound。
func GetSection[T any](m map[string]any, sectionName string) (*T, error) {
	sub := lookupMap(m, sectionName)
	if sub == nil {
		return nil, fmt.Errorf("%w: %s", ErrSectionNotFound, sectionName)
	}
	var out T
	if err := Bind(sub, &out); err != nil {
		return nil, fmt.Errorf("配置段 %s 绑定失败: %w", sectionName, err)
	}
	return &out, nil
}

// Bind 把 m 绑定到已存在的目标 out（组件 Init 的底座）：目标已有值保留，
// m 中出现的键覆盖（缺省即默认值的组件自治模式）。字段匹配 mapstructure
// tag 优先、无 tag 按完整字段名忽略大小写；WeaklyTypedInput 弱类型转换
// （"200" → int 等），并挂字符串时长钩子（"5s" → time.Duration）。
// 失败时返回原始错误，由调用方包装上下文。
func Bind(m map[string]any, out any) error {
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           out,
		WeaklyTypedInput: true,
		DecodeHook:       mapstructure.ComposeDecodeHookFunc(mapstructure.StringToTimeDurationHookFunc()),
	})
	if err == nil {
		err = dec.Decode(m)
	}
	return err
}

// lookupMap 沿点分路径逐层下钻取子 map：任一层 key 不存在或值不是
// map[string]any 即返回 nil。
func lookupMap(m map[string]any, path string) map[string]any {
	cur := m
	for _, seg := range strings.Split(path, ".") {
		next, ok := cur[seg].(map[string]any)
		if !ok {
			return nil
		}
		cur = next
	}
	return cur
}
