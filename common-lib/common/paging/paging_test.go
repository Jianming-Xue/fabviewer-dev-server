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
		in   PageRequest
		want PageRequest
	}{
		{"零值取默认", PageRequest{}, PageRequest{PageIndex: DefaultPage, PageSize: DefaultSize}},
		{"负值取默认", PageRequest{PageIndex: -3, PageSize: -1}, PageRequest{PageIndex: DefaultPage, PageSize: DefaultSize}},
		{"pageIndex 非法 pageSize 合法", PageRequest{PageIndex: 0, PageSize: 50}, PageRequest{PageIndex: 1, PageSize: 50}},
		{"pageSize 超上限截断", PageRequest{PageIndex: 2, PageSize: MaxPageSize + 1}, PageRequest{PageIndex: 2, PageSize: MaxPageSize}},
		{"边界值 pageSize=MaxPageSize 保留", PageRequest{PageIndex: 1, PageSize: MaxPageSize}, PageRequest{PageIndex: 1, PageSize: MaxPageSize}},
		{"合法值原样保留", PageRequest{PageIndex: 5, PageSize: 30}, PageRequest{PageIndex: 5, PageSize: 30}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.in.Normalize())
		})
	}

	t.Run("不修改原值", func(t *testing.T) {
		q := PageRequest{PageIndex: 0, PageSize: 0}
		_ = q.Normalize()
		assert.Equal(t, PageRequest{}, q)
	})
}

func TestLimitOffset(t *testing.T) {
	tests := []struct {
		name   string
		q      PageRequest
		limit  int
		offset int
	}{
		{"零值", PageRequest{}, DefaultSize, 0},
		{"第 2 页", PageRequest{PageIndex: 2, PageSize: 20}, 20, 20},
		{"第 5 页每页 10", PageRequest{PageIndex: 5, PageSize: 10}, 10, 40},
		{"pageIndex 非法按第 1 页", PageRequest{PageIndex: -2, PageSize: 25}, 25, 0},
		{"pageSize 非法取默认", PageRequest{PageIndex: 3}, DefaultSize, (3 - 1) * DefaultSize},
		{"pageSize 超上限截断后计算", PageRequest{PageIndex: 2, PageSize: 10000}, MaxPageSize, MaxPageSize},
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
		p := NewPageable[int](PageRequest{PageIndex: 1, PageSize: 20}, 0, nil)
		require.NotNil(t, p.Data)
		assert.Empty(t, p.Data)
		assert.Equal(t, int64(0), p.TotalSize)
		assert.Equal(t, int64(0), p.TotalPage, "totalSize 为 0 时总页数为 0")
	})

	t.Run("PageRequest 先 Normalize", func(t *testing.T) {
		p := NewPageable[string](PageRequest{}, 7, []string{"a"})
		assert.Equal(t, DefaultPage, p.PageIndex)
		assert.Equal(t, DefaultSize, p.PageSize)
	})

	t.Run("非空 data 原样保留", func(t *testing.T) {
		p := NewPageable[string](PageRequest{PageIndex: 2, PageSize: 2}, 5, []string{"a", "b"})
		assert.Equal(t, []string{"a", "b"}, p.Data)
		assert.Equal(t, int64(5), p.TotalSize)
	})
}

func TestTotalPage(t *testing.T) {
	tests := []struct {
		name      string
		total     int64
		pageSize  int
		totalPage int64
	}{
		{"不整除进一", 57, 10, 6},
		{"整除不加", 50, 10, 5},
		{"不足一页算一页", 3, 10, 1},
		{"零总数", 0, 10, 0},
		{"单条整页", 1, 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPageable[int](PageRequest{PageIndex: 1, PageSize: tt.pageSize}, tt.total, nil)
			assert.Equal(t, tt.totalPage, p.TotalPage)
		})
	}
}

func TestJSONSerialization(t *testing.T) {
	t.Run("空 Pageable 的 data 序列化为 []", func(t *testing.T) {
		b, err := json.Marshal(NewPageable[int](PageRequest{}, 0, nil))
		require.NoError(t, err)
		assert.JSONEq(t,
			`{"pageIndex":1,"pageSize":10,"totalSize":0,"totalPage":0,"data":[]}`, string(b))
	})

	t.Run("Pageable 字段名契约", func(t *testing.T) {
		b, err := json.Marshal(NewPageable[int](PageRequest{PageIndex: 3, PageSize: 15}, 100, []int{1, 2}))
		require.NoError(t, err)
		assert.JSONEq(t,
			`{"pageIndex":3,"pageSize":15,"totalSize":100,"totalPage":7,"data":[1,2]}`, string(b))
	})

	t.Run("PageRequest 字段名契约", func(t *testing.T) {
		b, err := json.Marshal(PageRequest{PageIndex: 2, PageSize: 30})
		require.NoError(t, err)
		assert.JSONEq(t, `{"pageIndex":2,"pageSize":30}`, string(b))
	})
}
