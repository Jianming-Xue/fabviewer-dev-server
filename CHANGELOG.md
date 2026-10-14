# Changelog

本库遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 格式，
版本号遵循语义化版本。业务系统按 git tag 锁定版本，升级依赖必须全量测试
通过后单独提交并记录于此。

## [Unreleased]

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
