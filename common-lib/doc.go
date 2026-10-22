// Package commonlib 是 fab-viewer 体系的 Go 公共库，被 RESTful API 业务与内部
// 定时器业务两类系统 import，集中提供存储访问（Oracle / ClickHouse / OpenSearch）
// 与 API 公共组件（统一响应、分页、错误处理、日志中间件）。
//
// 包按消费者组织，import 路径即能力边界，与业务系统的分层结构一一对应：
//
//	接口访问层  → api 按功能分包：request（请求参数解析/校验/时间/包装）、
//	              response（ApiResponse 统一响应 OK/Fail/VO→DTO）、
//	              api/middleware 按处理流程点分包（observer/errorhandler/
//	              guard/enrich）——仅此层可 import，Gin 依赖从这里开始
//	存储层      → storage/oracle、storage/clickhouse、storage/opensearch、
//	              storage/snowflake（ID 生成）、storage 根（PoolConfig）
//	所有层      → common/bizerror、common/paging、common/rctx（操作者并入 RuntimeContext.User）
//	              （零三方依赖的跨层包）、common/rctx（请求运行时上下文
//	              RuntimeContext）、common/i18n（错误消息国际化，只提供
//	              能力、译文业务自注册）
//
// 快速开始（业务系统 go.mod）：
//
//	replace github.com/fab-viewer/common-lib => ../../services/common-lib
//
// REST 接口层示例：
//
//	r.Use(observer.Observe(), errorhandler.ErrorHandler(),
//	      guard.Guard(), enrich.Enrich())
//	q := request.PageQuery(c)
//	response.OK(c, res)
//
// 定时器存储层示例（无接口层，不 import api）：
//
//	db, err := oracle.Open(ctx, cfg)
//	err = db.CallProc(ctx, "P_JOB", oracle.Out(&n))
//
// 详见各包文档与项目 README（含完整的包 ↔ 业务分层映射矩阵）。
package commonlib
