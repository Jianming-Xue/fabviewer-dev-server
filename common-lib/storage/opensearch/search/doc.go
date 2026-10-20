// Package search 提供 OpenSearch 泛型搜索：Search[T] 直出业务文档、
// WithPaging 消费 common/paging 分页值对象、WithSorts 生成排序 DSL；
// 错误翻译沿用根包约定（404→NotFound、4xx→Validation、5xx/传输→DB）。
package search
