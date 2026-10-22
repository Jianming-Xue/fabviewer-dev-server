package search

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"

	"github.com/fab-viewer/common-lib/common/bizerror"
	"github.com/fab-viewer/common-lib/common/commoncodes"
	"github.com/fab-viewer/common-lib/common/paging"
	opensearch "github.com/fab-viewer/common-lib/storage/opensearch"
)

// Hit 是单条搜索命中：ID 为文档 id，Score 为相关度得分，
// Source 为按泛型 T 反序列化后的业务文档。
type Hit[T any] struct {
	ID     string  `json:"_id"`
	Score  float64 `json:"_score"`
	Source T       `json:"_source"`
}

// SearchResult 是 Search 的返回：Total 为命中总数（不受分页影响），
// Hits 为当前页命中，保证非 nil（无结果时为空切片）。
//
//nolint:revive // 命名沿用领域词汇 SearchResult/SearchOption，分包后前缀重复可接受
type SearchResult[T any] struct {
	Total int64
	Hits  []Hit[T]
}

// searchConfig 汇聚 SearchOption 注入的可选项。
type searchConfig struct {
	page  *paging.PageRequest
	sorts []string
}

// SearchOption 是 Search 的可选项，分页与排序经它声明，保持 Search 参数不超限。
//
//nolint:revive // 同上，沿用领域词汇
type SearchOption func(*searchConfig)

// WithPaging 按分页值对象生成 DSL 的 from/size（q.Offset()/q.Limit()，
// 零值 Query 即默认第 1 页、每页 20 条）。
func WithPaging(q paging.PageRequest) SearchOption {
	return func(sc *searchConfig) { sc.page = &q }
}

// WithSorts 声明排序：每项形如 "-created_at"（降序）、"+id" 或 "id"（升序），
// 多字段按声明顺序生效；首字段取值相同的文档按后续字段稳定排序。
func WithSorts(sorts ...string) SearchOption {
	return func(sc *searchConfig) { sc.sorts = sorts }
}

// Search 在 index 上执行查询并返回类型化结果。query 为 ES 查询 DSL
// （如 map[string]any{"term": ...} 或业务侧查询结构体），序列化为请求体
// 的 query 字段，nil 表示 match_all。分页经 WithPaging、排序经 WithSorts。
// 错误翻译：404（index_not_found）→ commoncodes.ErrNotFound，4xx → CodeValidation，
// 5xx → CodeDB。
func Search[T any](ctx context.Context, c *opensearch.Client, index string, query any, opts ...SearchOption) (SearchResult[T], error) {
	if index == "" {
		return SearchResult[T]{}, bizerror.NewError(commoncodes.InvalidParameter, "opensearch.Search index 不能为空")
	}
	sc := searchConfig{}
	for _, opt := range opts {
		opt(&sc)
	}
	body, err := buildSearchBody(index, query, &sc)
	if err != nil {
		return SearchResult[T]{}, err
	}
	resp, err := c.DoSearch(ctx, &opensearchapi.SearchReq{
		Indices: []string{index},
		Body:    bytes.NewReader(body),
	})
	if err != nil {
		return SearchResult[T]{}, err
	}
	return decodeHits[T](index, resp)
}

// buildSearchBody 组装 /_search 请求体：query 序列化进 "query" 字段，
// 分页生成 from/size，排序生成 ES 对象形式（{"field":{"order":"desc"}}）。
func buildSearchBody(index string, query any, sc *searchConfig) ([]byte, error) {
	body := make(map[string]any, 3)
	if query != nil {
		body["query"] = query
	}
	if sc.page != nil {
		body["from"] = sc.page.Offset()
		body["size"] = sc.page.Limit()
	}
	if len(sc.sorts) > 0 {
		sorts, err := buildSorts(sc.sorts)
		if err != nil {
			return nil, bizerror.NewError(commoncodes.InvalidParameter, "opensearch.Search index={0} 构造排序", index).AttachError(err, nil)
		}
		body["sort"] = sorts
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, bizerror.NewError(commoncodes.InvalidParameter, "opensearch.Search index={0} 序列化查询 DSL", index).AttachError(err, nil)
	}
	return data, nil
}

// buildSorts 将 "-created_at"/"+id" 形式的排序声明翻译为 ES sort 数组，
// 空字段名（如 ""、"-"、"+"）视为非法声明返回错误。
func buildSorts(sorts []string) ([]any, error) {
	out := make([]any, 0, len(sorts))
	for _, s := range sorts {
		order, field := "asc", s
		switch {
		case strings.HasPrefix(s, "-"):
			order, field = "desc", strings.TrimPrefix(s, "-")
		case strings.HasPrefix(s, "+"):
			field = strings.TrimPrefix(s, "+")
		}
		if field == "" {
			return nil, bizerror.NewError(commoncodes.InvalidParameter, "opensearch.Search 非法排序项 {0}", s)
		}
		out = append(out, map[string]any{field: map[string]any{"order": order}})
	}
	return out, nil
}

// decodeHits 将 SDK SearchResp 的命中翻译为 SearchResult[T]。
func decodeHits[T any](index string, resp *opensearchapi.SearchResp) (SearchResult[T], error) {
	result := SearchResult[T]{
		Total: int64(resp.Hits.Total.Value),
		Hits:  make([]Hit[T], 0, len(resp.Hits.Hits)),
	}
	for i := range resp.Hits.Hits {
		sdkHit := &resp.Hits.Hits[i]
		hit := Hit[T]{ID: sdkHit.ID, Score: float64(sdkHit.Score)}
		if len(sdkHit.Source) > 0 {
			if err := json.Unmarshal(sdkHit.Source, &hit.Source); err != nil {
				return SearchResult[T]{}, bizerror.NewError(commoncodes.DBError, "opensearch.Search index={0} 反序列化命中 _id={1} 的 _source", index, sdkHit.ID).AttachError(err, nil)
			}
		}
		result.Hits = append(result.Hits, hit)
	}
	return result, nil
}
