# 当前状态（STATUS）

> 快照日期：2026-10-19。本文件记录仓库的实时状态与待办，历史沿革见
> [CHANGELOG](CHANGELOG.md)，完整规范见 [README](README.md)，交接说明见
> [HANDOVER](HANDOVER.md)。

## 一句话状态

三轮结构调整已完成并全量验证通过（目录重组 → API 契约重设计 → 存储层分包），
**全部变更尚未提交**；另有少量先前遗留的 WIP 问题（见「已知问题」）。

## 已完成（本工作区，未提交）

| 轮次 | 内容 | 验证 |
|---|---|---|
| 1. 目录重组 | 跨层包入 `common/`（errs/paging/logx/actor，现 actortx），snowflake 归 `storage/`，import 路径全量替换 | build/vet/test/lint 全过 |
| 2. API 契约重设计 | 统一响应体 `api.Response`（code=HTTP 状态码/success/errMsg/contents/txId 五字段恒序列化）；分页入参 `pageIndex/pageSize`、出参 `paging.Pageable`；`X-Request-Id`→`txId`；`RequestID`→`TxID` 符号全对齐；字符串错误码与 Details 退出响应体（转日志） | 单测全绿 + apifull 端到端实测 wire 格式 |
| 3. 运行时上下文 | 新增运行时上下文包（现 `common/rctx`，类型 `RuntimeContext`）；`middleware.FillRuntimeContext` 解析四 header（txId/txTime/txName/userName）三路注入（请求 context / actortx 操作者链路 / gin）；txTime 缺失或非法兜底服务器时间；userName 自动打通 oracle 审计字段 | 单测全绿 + 端到端实测（响应回显/日志字段） |
| 4. 存储层分包 | oracle 拆 `conn/config/base/repository` 四包（根只留向导文档）；clickhouse 拆 `config/batch`；opensearch 拆 `config/search`；SDK 装配逻辑归位连接包；新增逃生舱（`clickhouse.NewFromConn/WrapDB`、`opensearch.NewFromAPI/DoSearch`） | build/vet/test/lint 全过，mock 再生成无漂移 |
| 5. 请求国际化 | 新增 `common/i18n`（Bundles 模块多语言表 + Accept-Language 解析 + 回退链 + {0} 位置占位符，只提供能力、译文业务每模块自维护）与生成器 `cmd/i18nerr`（从模块内 `messages_<语言>.go` 生成 Code 常量/ErrorCode 字典句柄/构造函数，模块字典跟包走、无全局注册） | 单测全绿（i18n 覆盖率 100%）+ apifull 端到端实测（zh-CN/zh-TW 收窄/默认回退/生成器再生成零漂移） |

当前包结构：

```
common-lib/
├── common/          fverr / paging / actortx / rctx（跨层零依赖会合点）
│                    logx（zap 引擎完整日志工具：编号滚动/每出口自定义格式，仅白名单依赖 zap）
│                    i18n（多语言表能力，仅依赖 common/rctx）
├── cmd/i18nerr/     错误变量集合生成器（messages_<语言>.go → vars_gen.go）
├── api/             统一响应（apiresponse）、PageQuery、错误码映射
├── api/middleware/  AccessLog / ErrorHandler / Recovery / RuntimeContext
├── storage/
│   ├── pool.go      PoolConfig
│   ├── snowflake/   点分隔雪花 ID
│   ├── oracle/      根=向导；conn / config / base / repository
│   ├── clickhouse/  根=Client/Open/QuerySlice；config / batch
│   └── opensearch/  根=Client/Open/Index/Get/Delete；config / search
└── examples/        apifull / oracleproc / chbatch / ossearch（兼作编译烟雾测试）
```

## Git 状态（接手前必读）

- **2026-10-20 仓库重组**：git 根从本目录上移至上级 `services/`，本库成为
  其 `common-lib/` 子目录（历史经 rename 完整保留），与业务服务
  security-management 并列；仓库级交接见 `../HANDOVER.md`
- 上表 1–7 轮变更均已提交推送；最后 tag：`v0.5.1`（**Unreleased 已积累
  大量 breaking，接入业务前建议打 v0.6.0**）
- 快照日期更新为 2026-10-20；本文件"一句话状态"与实际同步

## 待定决策

- **txId 缺失行为**：header 未带 txId 时当前为「crypto/rand 生成 32 位 hex
  兜底」。规则待业务方拍板，改动点集中在 `api/middleware/runtimecontext.go`
  一处（注释已标注）

## 已知问题（均为先前遗留 WIP，非本轮引入）

| 问题 | 位置 | 影响 |
|---|---|---|
| 测试数据竞争：`nowFn` 钩子无同步写入 | `storage/snowflake/snowflake_test.go:134`（写）vs `snowflake.go:104`（读） | `make test`（-race）失败；需改 atomic |
| 单测覆盖率 86.8% | `storage/snowflake` | `make cover`（99% 门槛）不过 |
| 3 条 lint 告警 | snowflake×3（epochMs 未用、switch 可 tagged、real 遮蔽内建名）；actortx×1（nil ctx，SA1012）已在改名时修复 | `make lint` 非零退出（3 条） |
| `repository` 包无任何测试 | `storage/oracle/repository` | Create/Update 落库路径无保护（baseOf 校验可单测，落库需 sqlmock/集成） |

## 质量门槛现状

- `make lint`：除上述 4 条 WIP 告警外零告警
- `make test`：除 snowflake race 外全部通过（18 个有测试的包）
- `make cover`：受 snowflake 覆盖率拖累暂不过；其余包达标（跨层包与 api ≥99%，
  存储包装层 ≥95% 的分级目标不变）
- `make integration`：需要 docker compose 三容器（Oracle 初始化约 1-2 分钟），
  本轮结构调整后尚未跑过集成线（单测已覆盖装配逻辑，建议提交前补跑）
