// Command ossearch 演示 common-lib OpenSearch 封装的完整文档生命周期：
// Open → Index → Search（分页 + 排序）→ Get → Delete。
//
// 启动（匿名集群可省略 USER/PASS，内网自签集群加 INSECURE=1）：
//
//	COMMON_LIB_OS_ADDRS=http://localhost:9200 \
//	COMMON_LIB_OS_USER=admin COMMON_LIB_OS_PASS=admin \
//	go run ./examples/ossearch
//
// 未设置 COMMON_LIB_OS_ADDRS 时打印用法并以 0 退出（不视为失败）。
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fab-viewer/common-lib/errs"
	"github.com/fab-viewer/common-lib/paging"
	"github.com/fab-viewer/common-lib/storage/opensearch"
)

// order 是示例业务文档（对象层 DTO 的替身）。
type order struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

func main() {
	rawAddrs := strings.TrimSpace(os.Getenv("COMMON_LIB_OS_ADDRS"))
	if rawAddrs == "" {
		fmt.Println("未设置 COMMON_LIB_OS_ADDRS，用法：")
		fmt.Println("  COMMON_LIB_OS_ADDRS=http://localhost:9200 \\\n" +
			"  [COMMON_LIB_OS_USER=admin COMMON_LIB_OS_PASS=admin COMMON_LIB_OS_INSECURE=1] \\\n" +
			"  go run ./examples/ossearch")
		return
	}
	insecure, err := strconv.ParseBool(os.Getenv("COMMON_LIB_OS_INSECURE"))
	if err != nil {
		insecure = false // 未设置或非法值按 false 处理
	}

	ctx := context.Background()
	cli, err := opensearch.Open(opensearch.Config{
		Addrs:    strings.Split(rawAddrs, ","),
		Username: os.Getenv("COMMON_LIB_OS_USER"),
		Password: os.Getenv("COMMON_LIB_OS_PASS"),
		Insecure: insecure,
	})
	if err != nil {
		fail("Open 失败", err)
	}

	index := "common-lib-ossearch-demo"
	id := fmt.Sprintf("o-%d", time.Now().UnixNano())
	doc := order{
		ID:        id,
		Title:     "fab-viewer 示例订单",
		Status:    "paid",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// 1. Index：写入文档
	if err := cli.Index(ctx, index, id, doc); err != nil {
		fail("Index 失败", err)
	}
	fmt.Printf("Index ok: index=%s id=%s\n", index, id)

	// 2. Search：按 _id 精确过滤，带分页与排序
	res, err := opensearch.Search[order](ctx, cli, index,
		map[string]any{"term": map[string]any{"_id": id}},
		opensearch.WithPaging(paging.Query{Page: 1, Size: 10}),
		opensearch.WithSorts("-created_at", "+id"),
	)
	if err != nil {
		fail("Search 失败", err)
	}
	fmt.Printf("Search ok: total=%d hits=%d\n", res.Total, len(res.Hits))
	for _, hit := range res.Hits {
		fmt.Printf("  hit: _id=%s _score=%.2f title=%s created_at=%s\n",
			hit.ID, hit.Score, hit.Source.Title, hit.Source.CreatedAt)
	}

	// 3. Get：读取文档
	var got order
	found, err := opensearch.Get(ctx, cli, index, id, &got)
	if err != nil {
		fail("Get 失败", err)
	}
	if !found {
		fail("Get 失败", errs.New(errs.CodeNotFound, "文档未找到"))
	}
	fmt.Printf("Get ok: id=%s status=%s\n", got.ID, got.Status)

	// 4. Delete：删除文档（幂等，再次删除不报错）
	if err := cli.Delete(ctx, index, id); err != nil {
		fail("Delete 失败", err)
	}
	fmt.Printf("Delete ok: index=%s id=%s\n", index, id)
}

// fail 打印错误上下文后以非零码退出。
func fail(step string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", step, err)
	os.Exit(1)
}
