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
// 匹配（如 yml key "maxSize" 命中字段 MaxSize）。绑定开启
// WeaklyTypedInput，环境变量覆盖进入 map 的字符串可按目标字段类型转换。
// 路径未命中时返回 ErrSectionNotFound。
func GetSection[T any](m map[string]any, sectionName string) (*T, error) {
	sub := lookupMap(m, sectionName)
	if sub == nil {
		return nil, fmt.Errorf("%w: %s", ErrSectionNotFound, sectionName)
	}
	var out T
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           &out,
		WeaklyTypedInput: true,
	})
	if err == nil {
		err = dec.Decode(sub)
	}
	if err != nil {
		return nil, fmt.Errorf("配置段 %s 绑定失败: %w", sectionName, err)
	}
	return &out, nil
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
