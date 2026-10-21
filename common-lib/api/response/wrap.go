package response

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"

	"github.com/fab-viewer/common-lib/common/units"
)

// wireHook 把 VO 字段转成出参 wire 格式：time.Time → "yyyy-MM-dd HH:mm:ss"
// 字符串（units.FormatDateTime，零值为空串）、bool → int（units.BoolToInt）、
// 整数 → 字符串（如雪花 ID 字段）。目标同为 time.Time 时原样保留。
func wireHook(_, to reflect.Type, data any) (any, error) {
	if t, ok := data.(time.Time); ok && to != reflect.TypeOf(time.Time{}) {
		if t.IsZero() {
			return "", nil
		}
		return units.FormatDateTime(t), nil
	}
	if from := reflect.TypeOf(data); from != nil {
		switch {
		case from.Kind() == reflect.Bool && isIntKind(to.Kind()):
			return units.BoolToInt(data.(bool)), nil
		case isIntKind(from.Kind()) && to.Kind() == reflect.String:
			return strconv.FormatInt(reflect.ValueOf(data).Int(), 10), nil
		}
	}
	return data, nil
}

// isIntKind 判断是否整数族 kind（int/int8/.../int64）。
func isIntKind(k reflect.Kind) bool {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return true
	}
	return false
}

// Wrap 由 service 层 VO 生成 api 层 DTO：字段名一致（tag 优先，忽略大小写）
// 自动拷贝，wire 格式转换见 wireHook（time → "yyyy-MM-dd HH:mm:ss" 字符串、
// bool → int 1/0、整数 → 字符串）；其余类型不匹配时报错（不做弱转换，
// 错误尽早暴露）。服务层自行组装 DTO 的项目可只用 units 的格式原语，
// 不必经本方法。
func Wrap[DTO any](src any) (DTO, error) {
	var dto DTO
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(wireHook),
		Result:     &dto,
	})
	if err == nil {
		err = dec.Decode(toWireMap(src))
	}
	if err != nil {
		wrapErr := fmt.Errorf("response: wrap %T into %T: %w", src, &dto, err)
		return dto, wrapErr
	}
	return dto, nil
}

// toWireMap 把 VO 递归展开为 map[string]any（字段名 mapstructure/json tag
// 优先，未导出字段跳过；嵌套 struct、slice/map 内的 struct 同样展开；
// time.Time 原样保留由 wireHook 转换）。mapstructure 对 struct 输入会先
// 展开成中间 map 且把 time.Time 拆成内部字段（hook 无法拦截），先行展开
// 规避该路径。
func toWireMap(v any) map[string]any {
	rv := reflect.Indirect(reflect.ValueOf(v))
	out := make(map[string]any, rv.NumField())
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if f.PkgPath != "" {
			continue
		}
		out[fieldName(f.Name, f.Tag)] = toWireValue(rv.Field(i))
	}
	return out
}

// fieldName 取字段的目标名：mapstructure/json tag 优先（",omitempty" 等
// 选项去除、"-" 跳过），无 tag 用字段名。
func fieldName(name string, tag reflect.StructTag) string {
	for _, key := range []string{"mapstructure", "json"} {
		if v, ok := tag.Lookup(key); ok {
			if n, _, _ := strings.Cut(v, ","); n != "" && n != "-" {
				return n
			}
		}
	}
	return name
}

// toWireValue 递归展开值：指针解引用（nil 保持 nil）、struct 递归展开、
// slice/array 与 map 逐项展开，其余原样（time.Time 保留给 wireHook）。
func toWireValue(v reflect.Value) any {
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		return toWireValue(v.Elem())
	}
	switch v.Kind() {
	case reflect.Struct:
		if v.Type() == reflect.TypeOf(time.Time{}) {
			return v.Interface() // time 原样保留，由 wireHook 按目标类型转换
		}
		return toWireMap(v.Interface())
	case reflect.Slice, reflect.Array:
		items := make([]any, v.Len())
		for i := range items {
			items[i] = toWireValue(v.Index(i))
		}
		return items
	case reflect.Map:
		m := make(map[string]any, v.Len())
		iter := v.MapRange()
		for iter.Next() {
			m[fmt.Sprint(iter.Key().Interface())] = toWireValue(iter.Value())
		}
		return m
	default:
		return v.Interface()
	}
}
