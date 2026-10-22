//go:build integration

package opensearch_test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/stretchr/testify/require"

	"github.com/fab-viewer/common-lib/common/commoncodes"
	"github.com/fab-viewer/common-lib/common/paging"
	"github.com/fab-viewer/common-lib/storage/opensearch"
	"github.com/fab-viewer/common-lib/storage/opensearch/config"
	"github.com/fab-viewer/common-lib/storage/opensearch/search"
)

// TestOpenSearchLifecycle 是阶段 5 集成验收线：Open → Index → Search →
// Get → Delete。由环境变量驱动：COMMON_LIB_OS_ADDRS（必填，逗号分隔多地址）、
// COMMON_LIB_OS_USER / COMMON_LIB_OS_PASS / COMMON_LIB_OS_INSECURE（可选）；
// 缺少 COMMON_LIB_OS_ADDRS 时自动 skip。
func TestOpenSearchLifecycle(t *testing.T) {
	rawAddrs := strings.TrimSpace(os.Getenv("COMMON_LIB_OS_ADDRS"))
	if rawAddrs == "" {
		t.Skip("未设置 COMMON_LIB_OS_ADDRS，跳过 OpenSearch 集成测试")
	}
	insecure, _ := strconv.ParseBool(os.Getenv("COMMON_LIB_OS_INSECURE"))

	cli, err := opensearch.Open(config.Config{
		Addrs:    strings.Split(rawAddrs, ","),
		Username: os.Getenv("COMMON_LIB_OS_USER"),
		Password: os.Getenv("COMMON_LIB_OS_PASS"),
		Insecure: insecure,
	})
	require.NoError(t, err)

	ctx := context.Background()
	index := fmt.Sprintf("common-lib-ossearch-it-%d", time.Now().UnixMilli())
	id := fmt.Sprintf("doc-%d", time.Now().UnixNano())

	type order struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		CreatedAt string `json:"created_at"`
	}
	doc := order{ID: id, Title: "集成测试订单", CreatedAt: time.Now().UTC().Format(time.RFC3339)}

	// 测试索引用完即删（尽力而为，不阻断断言）。
	t.Cleanup(func() {
		_, _ = cli.SDK().Indices.Delete(ctx, opensearchapi.IndicesDeleteReq{Indices: []string{index}})
	})

	t.Run("Index", func(t *testing.T) {
		require.NoError(t, cli.Index(ctx, index, id, doc))
		// 刷新使写入对 _search 可见（Get 为实时读，无需刷新）。
		_, err := cli.SDK().Indices.Refresh(ctx, &opensearchapi.IndicesRefreshReq{Index: []string{index}})
		require.NoError(t, err)
	})

	t.Run("Search", func(t *testing.T) {
		res, err := search.Search[order](ctx, cli, index,
			map[string]any{"term": map[string]any{"_id": id}},
			search.WithPaging(paging.PageRequest{PageIndex: 1, PageSize: 10}),
			// 注：+id 不参与排序——源字段 id 是字符串，被 OpenSearch 动态映射为
			// text 类型（不可排序，需 keyword 映射）；created_at 动态映射为 date 可排序。
			search.WithSorts("-created_at"),
		)
		require.NoError(t, err)
		require.EqualValues(t, 1, res.Total)
		require.Len(t, res.Hits, 1)
		require.Equal(t, id, res.Hits[0].ID)
		require.Equal(t, doc.Title, res.Hits[0].Source.Title)
	})

	t.Run("Get", func(t *testing.T) {
		var got order
		found, err := opensearch.Get(ctx, cli, index, id, &got)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, doc.Title, got.Title)
	})

	t.Run("Delete 及删除后未命中", func(t *testing.T) {
		require.NoError(t, cli.Delete(ctx, index, id))

		var got order
		found, err := opensearch.Get(ctx, cli, index, id, &got)
		require.NoError(t, err)
		require.False(t, found)
	})

	t.Run("缺失索引搜索翻译为 NOT_FOUND", func(t *testing.T) {
		_, err := search.Search[order](ctx, cli, index+"-missing", nil)
		require.ErrorIs(t, err, commoncodes.ErrNotFound)
	})
}
