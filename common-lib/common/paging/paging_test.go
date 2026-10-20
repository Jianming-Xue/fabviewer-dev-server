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
		{"零值取默认", Query{}, Query{PageIndex: DefaultPage, PageSize: DefaultSize}},
		{"负值取默认", Query{PageIndex: -3, PageSize: -1}, Query{PageIndex: DefaultPage, PageSize: DefaultSize}},
		{"pageIndex 非法 pageSize 合法", Query{PageIndex: 0, PageSize: 50}, Query{PageIndex: 1, PageSize: 50}},
		{"pageSize 超上限截断", Query{PageIndex: 2, PageSize: MaxSize + 1}, Query{PageIndex: 2, PageSize: MaxSize}},
		{"边界值 pageSize=MaxSize 保留", Query{PageIndex: 1, PageSize: MaxSize}, Query{PageIndex: 1, PageSize: MaxSize}},
		{"合法值原样保留", Query{PageIndex: 5, PageSize: 30}, Query{PageIndex: 5, PageSize: 30}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.in.Normalize())
		})
	}

	t.Run("不修改原值", func(t *testing.T) {
		q := Query{PageIndex: 0, PageSize: 0}
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
		{"第 2 页", Query{PageIndex: 2, PageSize: 20}, 20, 20},
		{"第 5 页每页 10", Query{PageIndex: 5, PageSize: 10}, 10, 40},
		{"pageIndex 非法按第 1 页", Query{PageIndex: -2, PageSize: 25}, 25, 0},
		{"pageSize 非法取默认", Query{PageIndex: 3}, DefaultSize, (3 - 1) * DefaultSize},
		{"pageSize 超上限截断后计算", Query{PageIndex: 2, PageSize: 10000}, MaxSize, MaxSize},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.limit, tt.q.Limit())
			assert.Equal(t, tt.offset, tt.q.Offset())
		})
	}
}

func TestNewPageable(t *testing.T) {
	t.Run("nil data 置空切片", func(t *testing.T) {
		p := NewPageable[int](Query{PageIndex: 1, PageSize: 20}, 0, nil)
		require.NotNil(t, p.Data)
		assert.Empty(t, p.Data)
		assert.Equal(t, int64(0), p.TotalSize)
	})

	t.Run("Query 先 Normalize", func(t *testing.T) {
		p := NewPageable[string](Query{}, 7, []string{"a"})
		assert.Equal(t, DefaultPage, p.PageIndex)
		assert.Equal(t, DefaultSize, p.PageSize)
	})

	t.Run("非空 data 原样保留", func(t *testing.T) {
		p := NewPageable[string](Query{PageIndex: 2, PageSize: 2}, 5, []string{"a", "b"})
		assert.Equal(t, []string{"a", "b"}, p.Data)
		assert.Equal(t, int64(5), p.TotalSize)
	})
}

func TestJSONSerialization(t *testing.T) {
	t.Run("空 Pageable 的 data 序列化为 []", func(t *testing.T) {
		b, err := json.Marshal(NewPageable[int](Query{}, 0, nil))
		require.NoError(t, err)
		assert.JSONEq(t,
			`{"pageSize":20,"pageIndex":1,"totalSize":0,"data":[]}`, string(b))
	})

	t.Run("Pageable 字段名契约", func(t *testing.T) {
		b, err := json.Marshal(NewPageable[int](Query{PageIndex: 3, PageSize: 15}, 100, []int{1, 2}))
		require.NoError(t, err)
		assert.JSONEq(t,
			`{"pageSize":15,"pageIndex":3,"totalSize":100,"data":[1,2]}`, string(b))
	})

	t.Run("Query 字段名契约", func(t *testing.T) {
		b, err := json.Marshal(Query{PageIndex: 2, PageSize: 30})
		require.NoError(t, err)
		assert.JSONEq(t, `{"pageIndex":2,"pageSize":30}`, string(b))
	})
}
