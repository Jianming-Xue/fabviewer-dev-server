# Changelog

本库遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 格式，
版本号遵循语义化版本。业务系统按 git tag 锁定版本，升级依赖必须全量测试
通过后单独提交并记录于此。

## [Unreleased]

### Changed（Breaking：存储层分包）

- `storage/oracle` 按功能拆四包：`config`（Config/WithDefaults/Validate/BuildDSN）、
  `conn`（Open/DB/存储过程/Cursor/驱动适配）、`base`（BasePO 审计基类）、
  `repository`（Repository[T] 通用仓储）；根包只留分包向导文档
- `storage/clickhouse` 拆 `config` 与 `batch` 子包；`storage/opensearch` 拆
  `config` 与 `search` 子包
- 连接装配逻辑归位：clickhouse 的 Options 映射与 opensearch 的 SDK 配置装配
  移回各自根包（config 子包保持纯数据+校验）；oracle Config 的
  `validate`/`connectString` 导出为 `Validate`/`ConnectString`
- 新增逃生舱：`clickhouse.NewFromConn`（含错误包装 `clickhouse.WrapDB`）、
  `opensearch.NewFromAPI` 与 `opensearch.Client.DoSearch`
- 业务方 import 迁移对照：`oracle.Open`→`conn.Open`、`oracle.Config`→`config.Config`、
  `oracle.Out/OutCursor/CallProcTx/FromGorm/Cursor`→`conn.*`、`oracle.BasePO`→`base.BasePO`、
  `oracle.NewRepository`→`repository.NewRepository`、`clickhouse.NewBatch`→`batch.NewBatch`、
  `clickhouse.Config`→子包 `config.Config`、`opensearch.Search/WithPaging/WithSorts`→`search.*`

### Added

- 统一业务错误重构为 `common/fverr.FvError`（四字段 Code/Message/Err/Details，
  出口只输出 Message、其余全进日志）。API 收敛为唯一构造
  `NewError(code, message, args...)` + 链式 `InsertArgs`（按序替换 `{0}` `{1}`
  位置占位符，各语言占位符在句中位置不同不受影响——删除确认等需带实体名的
  消息即此模式）+ `AttachError`（补根因明细）+ `IsError`/`From`；业务层的多语言
  创建经模块构造函数（内部查模块字典后交给 NewError，见 cmd/i18nerr）。
  原 `common/errs` 包删除，迁移对照：`errs.New/Newf→fverr.NewError(code, msg
  [, args...])`、`errs.Wrap/Wrapf→fverr.NewError(...).AttachError(err, nil)`、
  `errs.Error→fverr.FvError`、`errs.Code→fverr.Code`（fmt 动词改为 {n} 占位符）、
  sentinel 同名保留；`api.DefaultStatusMap`/`WithStatusMap`/`WithMessageFunc`
  键类型随之改为 `fverr.Code`/`*fverr.FvError`
- 后端请求国际化 `common/i18n`：**只提供能力、不内置文案**，译文由业务服务
  每模块自行维护。`i18n.Bundles`（语言→key→模板，RWMutex 并发安全）+ 回退链
  （请求语言 → 基础语言只收窄不放宽 → 默认语言 → key 原文）+
  `ParseAcceptLanguage` 解析标准 Accept-Language 头（q 值降序、规范化）；
  模板占位符为位置式 `{0}` `{1}`。生成器 `cmd/i18nerr` 从模块内
  `messages_<语言>.go` 生成 `errors_gen.go`（Code 常量、模块字典句柄
  `ErrorCode`、每 key 一个构造函数），模块字典跟模块包走、**无全局注册**；
  出口（ErrorHandler）**不做翻译**，直出 Message（原 WithI18N 选项删除）。
  接入链路：`rctx.RuntimeContext.Lang` 承载请求语言（FillRuntimeContext
  解析 `Accept-Language` 注入）；`api` 新增 `HeaderAcceptLanguage` 常量与
  `api.LangOf` 语法糖；`WithMessageFunc` 保留为逃生舱口（返回空串保留原
  Message）
- 新增 [STATUS.md](STATUS.md)（仓库实时状态、git 快照、待定决策、已知问题、
  质量门槛现状）与 [HANDOVER.md](HANDOVER.md)（交接文档：设计理念、包地图、
  三大契约 wire 格式、开发验证流程、未完成事项与约定速记）
- 请求运行时上下文 `common/rctx.RuntimeContext`：请求头 txId/txTime
  （yyyyMMDDHHmmss，缺失或解析失败兜底服务器时间）/txName/userName 由
  `middleware.FillRuntimeContext` 组装为 `rctx.RuntimeContext` 注入请求
  context，业务层经 `rctx.GetRuntimeContext` 全链路读取；userName
  同时注入 actortx 操作者上下文（oracle BasePO 的 CreatedBy/UpdatedBy 自动填充）；
  访问日志新增 tx_name/user 字段

### Changed（Breaking：API 层契约重设计）

- 统一响应体重构为 apiresponse（`api.Response`）：`{code:int(HTTP 状态码), success:bool,
  errMsg:string, contents:any, txId:string}`，五字段恒序列化。原先的字符串业务码
  （`errs.Code`）、`message`/`data`/`details`/`requestId`/`timestamp` 字段移出响应体；
  `errs.Code` 转为纯内部映射键，Details 随 `business error` 日志留痕
- 事务 ID 通道变更：请求头 `X-Request-Id` → `txId`；`middleware.RequestID()`
  → `FillRuntimeContext()`（经 TxID 演进而来）；`api.HeaderRequestID/CtxRequestID/RequestIDOf`
  → `HeaderTxID/CtxRuntimeContext/RuntimeContextOf/TxIDOf`；访问日志字段 `request_id` → `tx_id`
  （txId 缺失时生成兜底的行为待定，暂保持生成）
- 运行时上下文命名对齐（概念名回归"RuntimeContext"）：包 `common/tx` → `common/rctx`、
  类型 `tx.Tx` → `rctx.RuntimeContext`、中间件工厂 `RuntimeContext()` →
  `FillRuntimeContext()`、`api.TxOf` → `api.RuntimeContextOf`、
  `api.CtxTx` → `api.CtxRuntimeContext`；业务迁移对照：
  `common-lib/common/tx` → `common-lib/common/rctx`、`tx.Tx{...}` →
  `rctx.RuntimeContext{...}`、`tx.NewContext/FromContext` → `rctx.SetRuntimeContext/FromContext`、
  `middleware.RuntimeContext()` → `middleware.FillRuntimeContext()`、
  `api.TxOf` → `api.RuntimeContextOf`
- 分页契约变更：入参 `page`/`size` → `pageIndex`/`pageSize`；出参 `paging.Result`
  → `paging.Pageable`（`pageSize/pageIndex/totalSize/data`），`NewResult` → `NewPageable`；
  `paging.Query` 字段 `Page`/`Size` → `PageIndex`/`PageSize`（`Limit()/Offset()/Normalize()`
  语义不变）
- 导出符号改名：`api.Body` → `api.Response`、`api.NewBody` → `api.NewResponse`；
  其余（`OK/Created/NoContent/OKPage/Fail/Write/PageQuery/StatusFor`、
  `WithStatusMap/WithMessageFunc/WithSkipPaths`）保持不变

### Changed（目录重组）

- 目录重组（结构清晰化，公共 API 符号零变化，仅 import 路径变更）：
  - 跨层基础包挪入 `common/`：`errs` → `common/errs`（后演化为 `common/fverr`）、`paging` → `common/paging`、
    `logx` → `common/logx`、`actor` → `common/actortx`
  - 雪花 ID 归属存储：`snowflake` → `storage/snowflake`（Oracle Repository 内部
    消费）；storage 根保留 `PoolConfig`
  - 业务系统升级需批量替换 import 路径（符号与调用点零改动），如：
    `sed -i '' -e 's|common-lib/errs|common-lib/common/errs|g' -e 's|common-lib/paging|common-lib/common/paging|g' -e 's|common-lib/logx|common-lib/common/logx|g' -e 's|common-lib/actor|common-lib/common/actor|g' -e 's|common-lib/snowflake|common-lib/storage/snowflake|g' $(git ls-files '*.go')`
    （Linux 去掉 `-i` 后的空串参数）

### Added

- README 新增强制性规范：对象层约定（DTO/VO/PO 三类对象的定义位置、tag 规则、
  PO 未导出由编译器强制隐藏存储细节、原生 SQL 直出 VO 的标准姿势、ClickHouse
  ch tag 权衡）与定时器业务适配清单（九项：分层裁剪、VO↔PO 直转、错误处理、
  日志、事务、优雅退出、depguard 调整等）

### Fixed

- 修复存储层分包（Unreleased 第 4 轮）遗漏更新的三处 integration 标签测试
  （常规 build/vet/lint 因构建标签跳过而未暴露，`go vet -tags=integration`
  才可见）：oracle/conn 的 `DefaultPort` 与缺失 `config` import、clickhouse
  的 `Config`/`NewBatch`、opensearch 的 `Config`/`Search`/`WithPaging`/
  `WithSorts`——均按迁移对照表改指向 config/batch/search 子包

## [0.5.1] - 2026-10-15

### Fixed

- Oracle 真实库（gvenzl/oracle-free 26ai）集成验收全链路通过，修复四处只有
  真环境才能暴露的问题：
  - `BuildDSN` 的 alterSession 值需双层引号（godror 二次 logfmt 解码会把含空格
    的 NLS 值拆散，ORA-02248），附 godror.ParseDSN 回环测试
  - goose 版本表存在性探测返回 '1'/'0' 字符串（godror.Number 进不了 bool）
  - 弃用 goose 的 Querier 适配层、自实现 7 方法 Store（适配层硬绑 Go bool，
    godror 按 DPI BOOLEAN 绑定写入 VARCHAR2 必败，且 Oracle DDL 隐式提交
    会残留空表触发 missing zero version migration）
  - GORM 命名策略 upperNamer：整体大写对齐 Oracle 无引号标识符惯例，
    业务模型无需逐字段写 column tag（默认小写引号生成 "id" 报 ORA-00904）

### Verified

- Oracle 验收线：Open → Ping → goose 迁移 → GORM CRUD → CallProc 标量 OUT →
  游标 OUT → GORM 事务内 CallProcTx → Close（golang 容器 + 镜像内客户端库运行）

## [0.5.0] - 2026-10-14

### Added

- 基准测试基线：clickhouse 包装层 QuerySlice（~30ns/行）与 Batch.Append 开销
- 层间隔离实证：五层结构模拟业务工程 + depguard 违规注入验证（README 模板同步修正）
- 全库总覆盖率门槛 99% 实测达标（99.1%）

### Changed

- opensearch-go v4.5.0 → v4.8.1（IndicesRefreshReq 字段 Indices→Index 已适配）

### Fixed

- depguard 规则语义错误：deny 是前缀匹配而非 glob，原"跨层零依赖"规则从未生效；
  改为 `listMode: strict` + `$gostd` 白名单并以违规探针实测验证

## [0.4.0] - 2026-10-14

### Added

- `storage/opensearch`：OpenSearch v4 SDK 封装（Index/Get[T]/Delete/Search[T]，
  WithPaging/WithSorts，错误翻译，SDK 隔离 + SDK() 逃生舱，覆盖率 100%）
- `examples/ossearch`

## [0.3.0] - 2026-10-14

### Added

- `storage/clickhouse`：ClickHouse 轻量封装（Open fail-fast、Exec/Query/Ping、
  Batch[T] 阈值自动发送、QuerySlice[T] 反射填充，覆盖率 98.1%）
- `storage.PoolConfig`（连接池四参数 + WithDefaults）
- `examples/chbatch`

## [0.2.0] - 2026-10-14

### Added

- `storage/oracle`：GORM（GORM 出口）/ 原生 SQL（SQL 出口）/ 存储过程与函数
  （CallProc/CallFunc + 事务变体 CallProcTx/CallFuncTx + FromGorm）三通道；
  Out/OutCursor 标记、Cursor 游标、gormlog 分级日志、driverAdapter 驱动缝
  （默认 godror，预留 go-ora）、goose 迁移驱动的集成测试（覆盖率 98.4%）
- `examples/oracleproc`
- 里程碑：定时器业务可完整接入（存储 + 跨层包，不依赖 Gin）

## [0.1.0] - 2026-10-14

### Added

- `fverr`：统一业务错误 FvError（Code/Message/Err/Details，errors.Is/As 全兼容）
- `paging`：分页值对象 Query/Result[T]（非法值修正、空 items 序列化为 `[]`）
- `logx`：最小 Logger 接口 + slog 适配 + Nop（SetDefault 并发安全）
- `api`：统一响应体 Body 与 OK/Created/NoContent/OKPage/Fail、PageQuery
- `api/middleware`：RequestID / AccessLog / Recovery / ErrorHandler 四中间件，
  推荐注册顺序 **AccessLog → ErrorHandler → Recovery → RequestID**
  （AccessLog 必须在 ErrorHandler 外层，否则错误路径记录的 status 不正确）
- `examples/apifull`：可运行的全家桶示例（含优雅停机）

### Fixed

- 中间件注册顺序缺陷：AccessLog 位于 ErrorHandler 内层时，错误路径按默认值
  记录 status 200、panic 请求丢失访问日志；已修正顺序并由集成测试回归锁定

## [0.1.0-rc] - 2026-10-14

### Added

- 工程骨架：go.mod（module github.com/fab-viewer/common-lib）、golangci-lint v2
  配置（含库内 depguard 架构防腐规则）、Makefile
- 跨层基础包 errs（后演化为 fverr）/paging/logx 完整实现与单测（覆盖率 100%）
