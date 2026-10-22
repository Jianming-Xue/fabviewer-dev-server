package request

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"

	"github.com/fab-viewer/common-lib/common/bizerror"
	"github.com/fab-viewer/common-lib/common/units"
)

// errBadTime 表示时间字符串长度不属于任何 wire 格式。
var errBadTime = errors.New("unrecognized time length")

// intToBoolHook 把 wire 布尔约定（int：1 → true，其余 → false，见
// units.Bool）应用到字段映射：DTO 的 int 字段 → VO 的 bool 字段按
// units.Bool 语义转换（WeaklyTypedInput 默认的「非零即 true」与本库
// 约定不一致，DecodeHook 先行覆盖）。
func intToBoolHook(from, to reflect.Type, data any) (any, error) {
	if from.Kind() == reflect.Int && to.Kind() == reflect.Bool {
		return units.Bool(data.(int)), nil
	}
	return data, nil
}

// timeParseHook 把 wire 时间字符串（8 位 yyyyMMdd / 14 位 yyyyMMddHHmmss /
// 13 位毫秒时间戳，按长度识别）应用到字段映射：DTO 的 string 字段 → VO
// 的 time.Time 字段自动解析；空串保持原样（目标零值）。非法格式报错
// （经 Wrap 逐字段名呈现）。
func timeParseHook(from, to reflect.Type, data any) (any, error) {
	if from.Kind() != reflect.String || to != reflect.TypeOf(time.Time{}) {
		return data, nil
	}
	s := data.(string)
	if s == "" {
		return time.Time{}, nil // 空串按可选参数语义产出零值
	}
	t, err := parseTimeByLength(s)
	if err != nil {
		return nil, fmt.Errorf("%w: %q", err, s)
	}
	return t, nil
}

// Wrap 把 api 层入参对象（DTO）包装成 service 层提供的 VO：字段名一致
// （忽略大小写）自动拷贝并做弱类型转换（"30" → int、int → bool 按
// units.Bool 约定等）；类型无法转换时返回错误。字段对不齐或需要组装
// 裁剪的，业务自行手写转换函数，不强求走本方法。
func Wrap[VO any](src any) (VO, error) {
	var vo VO
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		DecodeHook:       mapstructure.ComposeDecodeHookFunc(intToBoolHook, timeParseHook),
		Result:           &vo,
	})
	if err == nil {
		err = dec.Decode(src)
	}
	if err != nil {
		wrapErr := fmt.Errorf("request: wrap %T into %T: %w", src, &vo, err)
		return vo, wrapErr
	}
	return vo, nil
}

// Require 校验必填字符串参数：去首尾空白后为空串时返回 VALIDATION 错误
// （errMsg 携带参数名），否则返回去空白后的值。
func Require(name, v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", bizerror.NewError(bizerror.CodeValidation, "parameter {0} is required", name)
	}
	return v, nil
}
