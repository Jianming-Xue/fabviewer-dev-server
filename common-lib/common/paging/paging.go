package paging

// 分页默认值与硬上限。
const (
	// DefaultPage 是 PageIndex 未提供或非法时的默认页码。
	DefaultPage = 1
	// DefaultSize 是 PageSize 未提供或非法时的默认每页条数。
	DefaultSize = 10
	// MaxPageSize 是每页条数硬上限，防止深翻页拖垮存储。
	MaxPageSize = 100
)

// PageRequest 是分页请求值对象：接口层解析请求参数（pageIndex/pageSize）
// 得到，沿服务层/聚合层传递，存储层消费 Limit()/Offset()（SQL）或经
// opensearch.WithPaging 转 from/size。零值经 Normalize 后即为默认分页
// （第 1 页、每页 10 条）。
type PageRequest struct {
	PageIndex int `form:"pageIndex" json:"pageIndex"`
	PageSize  int `form:"pageSize" json:"pageSize"`
}

// Normalize 返回修正非法值后的副本：PageIndex<=0 取 DefaultPage，
// PageSize<=0 取 DefaultSize，PageSize>MaxPageSize 截断为 MaxPageSize。
func (q PageRequest) Normalize() PageRequest {
	n := q
	if n.PageIndex <= 0 {
		n.PageIndex = DefaultPage
	}
	switch {
	case n.PageSize <= 0:
		n.PageSize = DefaultSize
	case n.PageSize > MaxPageSize:
		n.PageSize = MaxPageSize
	}
	return n
}

// Limit 返回每页条数（Normalize 后的 PageSize）。
func (q PageRequest) Limit() int { return q.Normalize().PageSize }

// Offset 返回偏移量 (PageIndex-1)*PageSize（基于 Normalize 后的值）。
func (q PageRequest) Offset() int {
	n := q.Normalize()
	return (n.PageIndex - 1) * n.PageSize
}

// Pageable 是分页出参对象，作为统一响应体的 contents 输出；
// 经 NewPageable 构造后 Data 保证非 nil（JSON 序列化为 [] 而非 null）。
type Pageable[T any] struct {
	PageIndex int   `json:"pageIndex"`
	PageSize  int   `json:"pageSize"`
	TotalSize int64 `json:"totalSize"`
	// TotalPage 是总页数：totalSize/pageSize 向上取整（不整除进一，
	// 整除不加；totalSize 为 0 时为 0）。
	TotalPage int64 `json:"totalPage"`
	Data      []T   `json:"data"`
}

// NewPageable 构造分页出参：q 先 Normalize；data 为 nil 时置为空切片；
// TotalPage 按 totalSize 向上取整计算。
func NewPageable[T any](q PageRequest, total int64, data []T) Pageable[T] {
	n := q.Normalize()
	if data == nil {
		data = []T{}
	}
	return Pageable[T]{
		PageIndex: n.PageIndex,
		PageSize:  n.PageSize,
		TotalSize: total,
		TotalPage: (total + int64(n.PageSize) - 1) / int64(n.PageSize),
		Data:      data,
	}
}
