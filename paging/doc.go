// Package paging 提供分页值对象 Query 与结果集 Result[T]，是接口访问层与存储层
// 之间共享分页语义的唯一会合点（依赖倒置到零依赖值对象）。
//
// 接口层通过 api.PageQuery 解析请求参数得到 Query，沿服务层/聚合层传递，
// 存储层消费 Limit()/Offset()（SQL LIMIT/OFFSET）或经 WithPaging 传给
// OpenSearch 的 from/size。api 包与 storage 包互不感知，仅在本包相会。
//
// 本包零三方依赖。
package paging
