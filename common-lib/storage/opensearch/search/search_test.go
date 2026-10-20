package search

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/common/fverr"
	"github.com/fab-viewer/common-lib/common/paging"
)

// searchBody 取第 i 个请求的 /_search 请求体（已解析为 map）。
func searchBody(t *testing.T, ft *fakeTransport, i int) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(ft.request(t, i).Body), &body))
	return body
}

// TestSearchOption 验证选项直接写入 searchConfig。
func TestSearchOption(t *testing.T) {
	sc := searchConfig{}
	WithPaging(paging.Query{PageIndex: 2, PageSize: 5})(&sc)
	WithSorts("-created_at", "+id")(&sc)
	require.NotNil(t, sc.page)
	assert.Equal(t, 2, sc.page.PageIndex)
	assert.Equal(t, 5, sc.page.PageSize)
	assert.Equal(t, []string{"-created_at", "+id"}, sc.sorts)
}

// orderDoc 是搜索测试的业务文档替身（与根包 client_test 各自持有一份）。
type orderDoc struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	N     int    `json:"n"`
}

// TestSearch 走完整 SDK 链路验证搜索：hits.total/_id/_score/_source 解析、
// 分页 from/size 与排序生成、query DSL 序列化、错误翻译，响应体必被关闭。
func TestSearch(t *testing.T) {
	ctx := context.Background()
	hitsBody := `{
		"took": 4,
		"hits": {
			"total": {"value": 2, "relation": "eq"},
			"max_score": 1.5,
			"hits": [
				{"_index": "orders", "_id": "o-1", "_score": 1.5,
				 "_source": {"id": "o-1", "title": "a", "n": 1}},
				{"_index": "orders", "_id": "o-2", "_score": 0.25,
				 "_source": {"id": "o-2", "title": "b", "n": 2}}
			]
		}
	}`

	t.Run("命中解析与请求体生成", func(t *testing.T) {
		ft := &fakeTransport{status: http.StatusOK, body: hitsBody}
		cli := newFakeClient(t, ft)
		query := map[string]any{"term": map[string]any{"status": "paid"}}
		res, err := Search[orderDoc](ctx, cli, "orders", query,
			WithPaging(paging.Query{PageIndex: 3, PageSize: 10}),
			WithSorts("-created_at", "+id"))
		require.NoError(t, err)
		assert.EqualValues(t, 2, res.Total)
		require.Len(t, res.Hits, 2)
		assert.Equal(t, "o-1", res.Hits[0].ID)
		assert.InDelta(t, 1.5, res.Hits[0].Score, 0.0001)
		assert.Equal(t, orderDoc{ID: "o-1", Title: "a", N: 1}, res.Hits[0].Source)
		assert.Equal(t, "o-2", res.Hits[1].ID)
		assert.Equal(t, orderDoc{ID: "o-2", Title: "b", N: 2}, res.Hits[1].Source)

		req := ft.request(t, 0)
		assert.Equal(t, http.MethodPost, req.Method)
		assert.Equal(t, "/orders/_search", req.Path)

		body := searchBody(t, ft, 0)
		assert.Equal(t, query, body["query"])
		assert.EqualValues(t, 20, body["from"], "第 3 页每页 10 条 → from=20")
		assert.EqualValues(t, 10, body["size"])
		assert.Equal(t, []any{
			map[string]any{"created_at": map[string]any{"order": "desc"}},
			map[string]any{"id": map[string]any{"order": "asc"}},
		}, body["sort"])
		ft.assertAllClosed(t)
	})

	t.Run("nil query 省略 query 字段", func(t *testing.T) {
		ft := &fakeTransport{status: http.StatusOK, body: `{"hits":{"total":{"value":0},"hits":[]}}`}
		cli := newFakeClient(t, ft)
		res, err := Search[orderDoc](ctx, cli, "orders", nil)
		require.NoError(t, err)
		assert.EqualValues(t, 0, res.Total)
		require.NotNil(t, res.Hits, "无结果时 Hits 应为空切片而非 nil")
		assert.Empty(t, res.Hits)

		body := searchBody(t, ft, 0)
		assert.NotContains(t, body, "query")
		assert.NotContains(t, body, "from")
		assert.NotContains(t, body, "size")
		assert.NotContains(t, body, "sort")
		ft.assertAllClosed(t)
	})

	t.Run("404 index_not_found 翻译为 NOT_FOUND", func(t *testing.T) {
		ft := &fakeTransport{status: http.StatusNotFound, body: `{
			"error": {"root_cause": [{"type": "index_not_found_exception",
			                          "reason": "no such index [orders]", "index": "orders"}],
			          "type": "index_not_found_exception", "reason": "no such index [orders]"},
			"status": 404}`}
		cli := newFakeClient(t, ft)
		_, err := Search[orderDoc](ctx, cli, "orders", nil)
		require.ErrorIs(t, err, fverr.ErrNotFound)
		e, ok := fverr.From(err)
		require.True(t, ok)
		assert.Equal(t, fverr.CodeNotFound, e.Code)
		assert.Contains(t, err.Error(), "no such index [orders]")
		assert.Contains(t, err.Error(), "opensearch.Search index=orders")
		ft.assertAllClosed(t)
	})

	t.Run("400 带 ES 错误体翻译为 VALIDATION", func(t *testing.T) {
		ft := &fakeTransport{status: http.StatusBadRequest, body: `{
			"error": {"root_cause": [{"type": "query_shard_exception",
			                          "reason": "Failed to parse query"}],
			          "type": "search_phase_execution_exception", "reason": "all shards failed"},
			"status": 400}`}
		cli := newFakeClient(t, ft)
		_, err := Search[orderDoc](ctx, cli, "orders", map[string]any{"match": "bad-query"})
		require.ErrorIs(t, err, fverr.ErrValidation)
		assert.Contains(t, err.Error(), "query_shard_exception: Failed to parse query")
		ft.assertAllClosed(t)
	})

	t.Run("500 结构化错误体翻译为 DB", func(t *testing.T) {
		ft := &fakeTransport{status: http.StatusInternalServerError, body: `{
			"error": {"type": "internal_error", "reason": "node crashed"}, "status": 500}`}
		cli := newFakeClient(t, ft)
		_, err := Search[orderDoc](ctx, cli, "orders", nil)
		require.ErrorIs(t, err, fverr.ErrDB)
		ft.assertAllClosed(t)
	})

	t.Run("500 非 JSON 错误体翻译为 DB", func(t *testing.T) {
		ft := &fakeTransport{status: http.StatusInternalServerError, body: `cluster down`}
		cli := newFakeClient(t, ft)
		_, err := Search[orderDoc](ctx, cli, "orders", nil)
		require.ErrorIs(t, err, fverr.ErrDB)
		ft.assertAllClosed(t)
	})

	t.Run("429 翻译为 RATE_LIMITED", func(t *testing.T) {
		ft := &fakeTransport{status: http.StatusTooManyRequests, body: `{"error":"too many","status":429}`}
		cli := newFakeClient(t, ft)
		_, err := Search[orderDoc](ctx, cli, "orders", nil)
		require.ErrorIs(t, err, fverr.ErrTooMany)
		ft.assertAllClosed(t)
	})

	t.Run("网络错误翻译为 DB", func(t *testing.T) {
		ft := &fakeTransport{failErr: context.DeadlineExceeded}
		cli := newFakeClient(t, ft)
		_, err := Search[orderDoc](ctx, cli, "orders", nil)
		require.ErrorIs(t, err, fverr.ErrDB)
		assert.Contains(t, err.Error(), "opensearch.Search index=orders")
	})

	t.Run("命中 _source 反序列化失败翻译为 DB", func(t *testing.T) {
		ft := &fakeTransport{status: http.StatusOK, body: `{
			"hits": {"total": {"value": 1}, "hits": [
				{"_id": "o-9", "_source": {"id": 999}}
			]}}`}
		cli := newFakeClient(t, ft)
		_, err := Search[orderDoc](ctx, cli, "orders", nil)
		require.ErrorIs(t, err, fverr.ErrDB)
		assert.Contains(t, err.Error(), "_id=o-9")
		ft.assertAllClosed(t)
	})

	t.Run("非法排序项", func(t *testing.T) {
		ft := &fakeTransport{}
		cli := newFakeClient(t, ft)
		for _, sort := range []string{"", "-", "+"} {
			_, err := Search[orderDoc](ctx, cli, "orders", nil, WithSorts(sort))
			require.ErrorIs(t, err, fverr.ErrValidation, "排序项 %q 应被拒绝", sort)
		}
		assert.Equal(t, 0, ft.requestCount())
	})

	t.Run("query 序列化失败", func(t *testing.T) {
		ft := &fakeTransport{}
		cli := newFakeClient(t, ft)
		_, err := Search[orderDoc](ctx, cli, "orders", map[string]any{"fn": func() {}})
		require.ErrorIs(t, err, fverr.ErrValidation)
		assert.Equal(t, 0, ft.requestCount())
	})

	t.Run("空 index 拒绝", func(t *testing.T) {
		ft := &fakeTransport{}
		cli := newFakeClient(t, ft)
		_, err := Search[orderDoc](ctx, cli, "", nil)
		require.ErrorIs(t, err, fverr.ErrValidation)
		assert.Equal(t, 0, ft.requestCount())
	})
}

// TestSearch_PagingNormalize 验证零值/非法分页经 paging 归一化后生成默认 from/size。
func TestSearch_PagingNormalize(t *testing.T) {
	ft := &fakeTransport{status: http.StatusOK, body: `{"hits":{"total":{"value":0},"hits":[]}}`}
	cli := newFakeClient(t, ft)
	_, err := Search[orderDoc](context.Background(), cli, "orders", nil, WithPaging(paging.Query{}))
	require.NoError(t, err)

	body := searchBody(t, ft, 0)
	assert.EqualValues(t, 0, body["from"], "第 1 页 → from=0")
	assert.EqualValues(t, 20, body["size"], "默认每页 20 条")
}
