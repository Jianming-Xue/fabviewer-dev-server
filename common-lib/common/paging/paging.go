package paging

// 分页默认值与硬上限。
const (
	// DefaultPage 是 PageIndex 未提供或非法时的默认页码。
	DefaultPage = 1
	// DefaultSize 是 PageSize 未提供或非法时的默认每页条数。
	DefaultSize = 20
	// MaxSize 是每页条数硬上限，防止深翻页拖垮存储。
	MaxSize = 500
)

// Query 是分页请求值对象：接口层解析请求参数（pageIndex/pageSize）得到，
// 沿服务层/聚合层传递，存储层消费 Limit()/Offset()（SQL）或经
// opensearch.WithPaging 转 from/size。零值经 Normalize 后即为默认分页
// （第 1 页、每页 20 条）。
type Query struct {
	PageIndex int `form:"pageIndex" json:"pageIndex"`
	PageSize  int `form:"pageSize" json:"pageSize"`
}

// Normalize 返回修正非法值后的副本：PageIndex<=0 取 DefaultPage，
// PageSize<=0 取 DefaultSize，PageSize>MaxSize 截断为 MaxSize。
func (q Query) Normalize() Query {
	n := q
	if n.PageIndex <= 0 {
		n.PageIndex = DefaultPage
	}
	switch {
	case n.PageSize <= 0:
		n.PageSize = DefaultSize
	case n.PageSize > MaxSize:
		n.PageSize = MaxSize
	}
	return n
}

// Limit 返回每页条数（Normalize 后的 PageSize）。
func (q Query) Limit() int { return q.Normalize().PageSize }

// Offset 返回偏移量 (PageIndex-1)*PageSize（基于 Normalize 后的值）。
func (q Query) Offset() int {
	n := q.Normalize()
	return (n.PageIndex - 1) * n.PageSize
}

// Pageable 是分页出参对象，作为统一响应体的 contents 输出；
// 经 NewPageable 构造后 Data 保证非 nil（JSON 序列化为 [] 而非 null）。
type Pageable[T any] struct {
	PageSize  int   `json:"pageSize"`
	PageIndex int   `json:"pageIndex"`
	TotalSize int64 `json:"totalSize"`
	Data      []T   `json:"data"`
}

// NewPageable 构造分页出参：q 先 Normalize；data 为 nil 时置为空切片。
func NewPageable[T any](q Query, total int64, data []T) Pageable[T] {
	n := q.Normalize()
	if data == nil {
		data = []T{}
	}
	return Pageable[T]{PageSize: n.PageSize, PageIndex: n.PageIndex, TotalSize: total, Data: data}
}
