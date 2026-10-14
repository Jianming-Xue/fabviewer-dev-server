# Changelog

本库遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 格式，
版本号遵循语义化版本。业务系统按 git tag 锁定版本，升级依赖必须全量测试
通过后单独提交并记录于此。

## [Unreleased]

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

- `errs`：统一业务错误码体系（Code/Error/sentinel/Wrap，errors.Is/As 全兼容）
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
- 跨层基础包 errs/paging/logx 完整实现与单测（覆盖率 100%）
