// Package observer 提供访问日志中间件（访问观察者）：每次请求记录一条含 method/path/
// status/latency_ms/client_ip/tx_id/tx_name/user/bytes 的 access 日志。
// 必须注册在 fault 外层，否则错误路径的状态码（404/500）会按
// gin 默认值 200 记录（见 api/middleware 包文档的顺序契约）。
package observer
