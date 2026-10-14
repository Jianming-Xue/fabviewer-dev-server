package paging

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		in   Query
		want Query
	}{
		{"零值取默认", Query{}, Query{Page: DefaultPage, Size: DefaultSize}},
		{"负值取默认", Query{Page: -3, Size: -1}, Query{Page: DefaultPage, Size: DefaultSize}},
		{"page 非法 size 合法", Query{Page: 0, Size: 50}, Query{Page: 1, Size: 50}},
		{"size 超上限截断", Query{Page: 2, Size: MaxSize + 1}, Query{Page: 2, Size: MaxSize}},
		{"边界值 size=MaxSize 保留", Query{Page: 1, Size: MaxSize}, Query{Page: 1, Size: MaxSize}},
		{"合法值原样保留", Query{Page: 5, Size: 30}, Query{Page: 5, Size: 30}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.in.Normalize())
		})
	}

	t.Run("不修改原值", func(t *testing.T) {
		q := Query{Page: 0, Size: 0}
		_ = q.Normalize()
		assert.Equal(t, Query{}, q)
	})
}

func TestLimitOffset(t *testing.T) {
	tests := []struct {
		name   string
		q      Query
		limit  int
		offset int
	}{
		{"零值", Query{}, DefaultSize, 0},
		{"第 2 页", Query{Page: 2, Size: 20}, 20, 20},
		{"第 5 页每页 10", Query{Page: 5, Size: 10}, 10, 40},
		{"page 非法按第 1 页", Query{Page: -2, Size: 25}, 25, 0},
		{"size 非法取默认", Query{Page: 3}, DefaultSize, (3 - 1) * DefaultSize},
		{"size 超上限截断后计算", Query{Page: 2, Size: 10000}, MaxSize, MaxSize},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.limit, tt.q.Limit())
			assert.Equal(t, tt.offset, tt.q.Offset())
		})
	}
}

func TestNewResult(t *testing.T) {
	t.Run("nil items 置空切片", func(t *testing.T) {
		r := NewResult[int](Query{Page: 1, Size: 20}, 0, nil)
		require.NotNil(t, r.Items)
		assert.Empty(t, r.Items)
		assert.Equal(t, int64(0), r.Total)
	})

	t.Run("Query 先 Normalize", func(t *testing.T) {
		r := NewResult[string](Query{}, 7, []string{"a"})
		assert.Equal(t, DefaultPage, r.Page)
		assert.Equal(t, DefaultSize, r.Size)
	})

	t.Run("非空 items 原样保留", func(t *testing.T) {
		r := NewResult[string](Query{Page: 2, Size: 2}, 5, []string{"a", "b"})
		assert.Equal(t, []string{"a", "b"}, r.Items)
		assert.Equal(t, int64(5), r.Total)
	})
}

func TestJSONSerialization(t *testing.T) {
	t.Run("空 Result 的 items 序列化为 []", func(t *testing.T) {
		b, err := json.Marshal(NewResult[int](Query{}, 0, nil))
		require.NoError(t, err)
		assert.JSONEq(t,
			`{"total":0,"page":1,"size":20,"items":[]}`, string(b))
	})

	t.Run("Result 字段名契约", func(t *testing.T) {
		b, err := json.Marshal(NewResult[int](Query{Page: 3, Size: 15}, 100, []int{1, 2}))
		require.NoError(t, err)
		assert.JSONEq(t,
			`{"total":100,"page":3,"size":15,"items":[1,2]}`, string(b))
	})

	t.Run("Query 字段名契约", func(t *testing.T) {
		b, err := json.Marshal(Query{Page: 2, Size: 30})
		require.NoError(t, err)
		assert.JSONEq(t, `{"page":2,"size":30}`, string(b))
	})
}
