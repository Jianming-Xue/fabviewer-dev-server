// Package commonlib 是 fab-viewer 体系的 Go 公共库，被 RESTful API 业务与内部
// 定时器业务两类系统 import，集中提供存储访问（Oracle / ClickHouse / OpenSearch）
// 与 API 公共组件（统一响应、分页、错误处理、日志中间件）。
//
// 包按消费者组织，import 路径即能力边界，与业务系统的分层结构一一对应：
//
//	接口访问层  → api、api/middleware（仅此层可 import，Gin 依赖从这里开始）
//	存储层      → storage/oracle、storage/clickhouse、storage/opensearch
//	所有层      → errs、paging、logx（零三方依赖的跨层包）
//
// 快速开始（业务系统 go.mod）：
//
//	replace github.com/fab-viewer/common-lib => ../../services/common-lib
//
// REST 接口层示例：
//
//	r.Use(middleware.ErrorHandler(log), middleware.Recovery(log),
//	      middleware.AccessLog(log), middleware.RequestID())
//	q := api.PageQuery(c)
//	api.OKPage(c, res)
//
// 定时器存储层示例（无接口层，不 import api）：
//
//	db, err := oracle.Open(ctx, cfg)
//	err = db.CallProc(ctx, "P_JOB", oracle.Out(&n))
//
// 详见各包文档与项目 README（含完整的包 ↔ 业务分层映射矩阵）。
package commonlib
