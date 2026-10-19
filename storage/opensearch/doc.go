// Package opensearch 提供 OpenSearch 访问封装（基于 opensearch-go/v4 官方
// SDK），仅可被业务系统的存储层（及组装根 main）import。
//
// 能力：连接装配 Open、泛型搜索 Search[T]（直接消费 paging.Query 分页值对象，
// 分页语义的跨层会合点）、文档 Index/Get/Delete。SDK 类型被隔离在包内，
// 需要本包未覆盖的原生能力（聚合、Bulk、索引管理等）时经 SDK() 逃生舱访问。
//
// 用法示例：
//
//	cli, err := opensearch.Open(oscfg.Config{
//	    Addrs:     []string{"https://os1:9200"},
//	    Username:  "admin",
//	    Password:  "admin",
//	    Insecure:  false,               // 内网自签集群才开启
//	})
//	...
//	_ = cli.Index(ctx, "orders", "o-1", order)
//	res, err := search.Search[order](ctx, cli, "orders",
//	    map[string]any{"term": map[string]any{"status": "paid"}},
//	    search.WithPaging(paging.Query{PageIndex: 2, PageSize: 10}),
//	    search.WithSorts("-created_at", "+id"))
//	found, err := opensearch.Get[order](ctx, cli, "orders", "o-1", &doc)
//	_ = cli.Delete(ctx, "orders", "o-1")
//
// 错误翻译约定：404（index_not_found）→ errs.ErrNotFound、4xx（含 ES 错误体）
// → CodeValidation、5xx 及传输层错误 → CodeDB，均带 "opensearch.XXX index=... id=..."
// 式操作上下文。本包不创建任何后台 goroutine。
package opensearch
