package paging

// 分页默认值与硬上限。
const (
	// DefaultPage 是 Page 未提供或非法时的默认页码。
	DefaultPage = 1
	// DefaultSize 是 Size 未提供或非法时的默认每页条数。
	DefaultSize = 20
	// MaxSize 是每页条数硬上限，防止深翻页拖垮存储。
	MaxSize = 500
)

// Query 是分页请求值对象：接口层解析请求参数得到，沿服务层/聚合层传递，
// 存储层消费 Limit()/Offset()（SQL）或经 opensearch.WithPaging 转 from/size。
// 零值经 Normalize 后即为默认分页（第 1 页、每页 20 条）。
type Query struct {
	Page int `form:"page" json:"page"`
	Size int `form:"size" json:"size"`
}

// Normalize 返回修正非法值后的副本：Page<=0 取 DefaultPage，
// Size<=0 取 DefaultSize，Size>MaxSize 截断为 MaxSize。
func (q Query) Normalize() Query {
	n := q
	if n.Page <= 0 {
		n.Page = DefaultPage
	}
	switch {
	case n.Size <= 0:
		n.Size = DefaultSize
	case n.Size > MaxSize:
		n.Size = MaxSize
	}
	return n
}

// Limit 返回每页条数（Normalize 后的 Size）。
func (q Query) Limit() int { return q.Normalize().Size }

// Offset 返回偏移量 (Page-1)*Size（基于 Normalize 后的值）。
func (q Query) Offset() int {
	n := q.Normalize()
	return (n.Page - 1) * n.Size
}

// Result 是分页结果集，经 NewResult 构造后 Items 保证非 nil
// （JSON 序列化为 [] 而非 null）。
type Result[T any] struct {
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Size  int   `json:"size"`
	Items []T   `json:"items"`
}

// NewResult 构造分页结果：q 先 Normalize；items 为 nil 时置为空切片。
func NewResult[T any](q Query, total int64, items []T) Result[T] {
	n := q.Normalize()
	if items == nil {
		items = []T{}
	}
	return Result[T]{Total: total, Page: n.Page, Size: n.Size, Items: items}
}
