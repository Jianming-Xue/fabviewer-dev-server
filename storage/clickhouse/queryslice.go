package clickhouse

import (
	"context"
	"reflect"

	"github.com/fab-viewer/common-lib/errs"
)

// querySliceInitialCap 是结果行数未知时的初始切片容量：ClickHouse 数据块
// 通常为数百行至数十万行，取 64 作为兼顾空查询与小结果集的起步值。
const querySliceInitialCap = 64

// QuerySlice 执行查询并把每行按 ch tag 反射填充为 T，返回 []T。
// T 必须为结构体（否则返回 errs.CodeDB 错误，不发起查询）；查询结果列在
// T 中没有对应 ch tag/同名字段（列缺失），或列类型与字段类型不匹配时，
// 返回 errs.CodeDB 错误并定位到行号。空结果返回长度 0 的非 nil 切片；
// 反射路径在包边界 recover，不向业务方泄漏 panic。
func QuerySlice[T any](ctx context.Context, c *Client, query string, args ...any) (items []T, err error) {
	if kind := reflect.TypeFor[T]().Kind(); kind != reflect.Struct {
		return nil, errs.Newf(errs.CodeDB, "clickhouse.QuerySlice query=%s: 类型参数 %s 非结构体",
			queryPreview(query), kind)
	}
	defer func() { // 反射路径在包边界 recover
		if r := recover(); r != nil {
			items, err = nil, errs.Newf(errs.CodeDB, "clickhouse.QuerySlice query=%s: %v",
				queryPreview(query), r)
		}
	}()
	rows, err := c.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, errs.Wrapf(err, errs.CodeDB, "clickhouse.QuerySlice query=%s", queryPreview(query))
	}
	defer func() { _ = rows.Close() }()
	items = make([]T, 0, querySliceInitialCap)
	for rows.Next() {
		var v T
		if err := rows.ScanStruct(&v); err != nil {
			return nil, errs.Wrapf(err, errs.CodeDB, "clickhouse.QuerySlice query=%s 第 %d 行填充失败",
				queryPreview(query), len(items)+1)
		}
		items = append(items, v)
	}
	if err := rows.Err(); err != nil {
		return nil, errs.Wrapf(err, errs.CodeDB, "clickhouse.QuerySlice query=%s", queryPreview(query))
	}
	return items, nil
}
