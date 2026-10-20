# 交接文档（HANDOVER）

> 面向接手本库的工程师：项目是什么、怎么组织的、三大契约长什么样、
> 怎么开发验证、还欠什么。规范细节以 [README](README.md) 为准，
> 实时状态与待办见 [STATUS](STATUS.md)，变更史见 [CHANGELOG](CHANGELOG.md)。

## 1. 项目是什么

fab-viewer 体系的 Go 公共库（module：`github.com/fab-viewer/common-lib`，
module 名为占位、业务方经 `replace` 本地引用；2026-10-20 起 git 根上移至
上级 `services/`，本库为其中的 `common-lib/` 子目录，与业务服务
security-management 并列，仓库级交接见上级 [HANDOVER](../HANDOVER.md)）。
被两类业务系统 import：

- **RESTful API 业务**（Gin）：用 `api` + `api/middleware` + 存储包
- **内部定时器业务**（无接口层）：只用跨层包 + 存储包

硬约束：**零后台 goroutine**、全同步调用路径、import 路径即能力边界。

## 2. 核心设计理念（务必先懂再动）

1. **import 路径即能力边界**：`api`/`api/middleware` 只准接口访问层 import
   （Gin 依赖从这里开始）；`storage/*` 只准存储层 import；`common/*` 跨层通用。
   由 `.golangci.yml` depguard 强制（deny 是前缀匹配，能命中全部子包）。
2. **跨层会合点**：`api` 与 `storage` 互不感知，只在小包相会——
   `fverr`（统一错误 FvError）、`paging`（分页对象）、`actortx`（操作者）、`rctx`（运行时上下文）。
   需要两边共享的概念，加包而不是加依赖。
3. **DTO/VO/PO 对象流转**：DTO 只在 api 层进出；VO 是 storage 对上的唯一边界
   类型（禁止 gorm/ch tag）；PO 未导出锁死在业务存储包内（编译器强制）。
   详见 README「对象层约定」。
4. **配置与装配分离**（本轮确立）：`*/config` 子包是纯数据+校验；
   SDK Options/DSN 之类的装配逻辑在连接包。
5. **逃生舱模式**：包装层不遮蔽 SDK——`clickhouse.Conn()/NewFromConn/WrapDB`、
   `opensearch.SDK()/NewFromAPI/DoSearch`、`oracle DB.GORM()/SQL()`。

## 3. 包地图

```
common/fverr    FvError（唯一构造 NewError(code,msg,args...)，链式 InsertArgs/AttachError，IsError/From）
common/paging    Query{PageIndex,PageSize}、Pageable[T]（出参 pageable）
common/logx      zap 引擎完整工具（stdout/文件编号滚动/远程批量出口，每出口可注入自定义格式）+ Logger 接口接缝
common/actortx   操作者 context 记录与传递（审计字段的来源；SetActorContext/GetActorContext）
common/rctx      RuntimeContext{ID,Time,Name,User,Lang} 运行时上下文（请求头四字段 + Accept-Language）
common/i18n      Bundles 模块多语言表 + Accept-Language 解析（只提供能力，译文业务每模块自维护；仅依赖 rctx）
api              Response/OK/OKPage/Fail、PageQuery、DefaultStatusMap
api/middleware   AccessLog → ErrorHandler → Recovery → FillRuntimeContext（顺序固定）
storage/oracle/conn        Open、DB 三通道（GORM/SQL/存储过程）、Cursor、driverAdapter
storage/oracle/config      Config/WithDefaults/Validate/BuildDSN
storage/oracle/base        BasePO 审计基类（GORM 钩子自动填 CreatedBy 等）
storage/oracle/repository  Repository[T]（雪花 ID + 审计落库）
storage/clickhouse(+config/batch)   Open/Client/QuerySlice；Batch[T] 批量写
storage/opensearch(+config/search/logsink)  Open/Client/Index/Get/Delete；Search[T]；logsink 日志直写
storage/snowflake  点分隔雪花 ID（"<Unix毫秒>.<节点>.<序列>"）
storage/pool.go    PoolConfig 连接池配置
cmd/i18nerr        模块错误字典生成器（messages_<语言>.go → errors_gen.go）
examples/          apifull（含 i18n 完整示例）/ oracleproc / chbatch / ossearch
```

依赖方向（无环）：config ← conn ← repository；base 只依赖 actortx+gorm；
batch/search 依赖各自根包；所有存储子包互不依赖（depguard 强制）。

## 4. 三大对外契约（wire 格式）

### 4.1 统一响应 apiresponse（五字段恒序列化，成功/失败同构）

```json
{"code":200,"success":true,"errMsg":"","contents":{...},"txId":"abc"}
{"code":404,"success":false,"errMsg":"user 999 not found","contents":null,"txId":"abc"}
```

- `code` = HTTP 状态码（数字），由 `fverr.Code → DefaultStatusMap` 映射产生
- 字符串错误码与 Details **不出参**，随 `business error` 日志留痕
- 业务错误统一走 `c.Error(err)` → ErrorHandler，handler 不自写错误响应

### 4.2 分页

- 入参：`?pageIndex=2&pageSize=20`（`api.PageQuery` 解析；非法回退 1/20，
  pageSize 上限 500）
- 出参（置于 contents）：`{"pageSize":20,"pageIndex":2,"totalSize":57,"data":[...]}`

### 4.3 请求运行时上下文

请求头四字段 + `Accept-Language` → `middleware.FillRuntimeContext` →
`rctx.RuntimeContext` 三路注入：

| header | RuntimeContext | 兜底 |
|---|---|---|
| txId | ID | 缺失生成 32 hex（**行为待定**，改 `runtimecontext.go` 一处） |
| txTime | Time | yyyyMMDDHHmmss；缺失/非法→服务器时间 |
| txName | Name | 可空 |
| userName | User | 可空；同时注入 actortx → oracle BasePO 审计自动填充 |
| Accept-Language | Lang | q 值降序取首选并规范化；缺失/非法为空串 |

业务层读取：`rctx.GetRuntimeContext(c.Request.Context())`。

### 4.4 错误消息国际化

`common/i18n` 只提供能力、译文由业务服务自注册；errMsg 按请求语言翻译：

- 接入（每模块自己的多语言表，编译期进包、无全局注册）：模块内
  `messages_<语言>.go`（key 即 code，{0} {1} 位置占位符）→
  `cmd/i18nerr -dir . -pkg xxx -default en` 生成 `errors_gen.go`（Code 常量、
  ErrorCode 字典句柄、构造函数）→ 调用处 `xxx.NewUserNotFound(ctx).
  InsertArgs(id).AttachError(root, details)`——构造函数内部按请求语言查模板后交给
  `fverr.NewError`；出口直出 Message 不翻译（`ErrorHandler` 已无 WithI18N）
- 回退链：请求语言 → 基础语言（只收窄不放宽）→ 默认语言 → key 原文
  （模块各语言文件经生成器注册，未命中 key 极少发生，发生即配置缺失信号）
- 删除确认等带实体名的消息：模板 `{0}` + `InsertArgs(name)`（角色名/用户名/
  登录策略名同理，各语言占位符位置不同不受影响，见 examples/apifull 的
  DELETE /api/users/:id）；模块码经 `WithStatusMap` 映射状态码

## 5. 开发与验证

```bash
make lint         # golangci-lint v2（depguard 层间隔离）
make test         # go test -race（CI 必含 race）
make cover        # 覆盖率门槛 99%（分级：跨层/api ≥99%，存储 ≥95%）
make mock         # go generate（oracle/conn 的 mockgen）
make fmt          # 提交前跑
make integration  # 集成测试：先 docker compose -f docker-compose.test.yml up -d
```

集成测试环境变量：`COMMON_LIB_ORACLE_DSN` / `COMMON_LIB_CH_DSN` /
`COMMON_LIB_OS_ADDRS`，缺失自动 skip。goose 迁移样板在
`storage/oracle/conn/testdata/migrations/oracle/`。

日常自证（不连库）：`go run ./examples/apifull` + curl（文件头有现成命令，
含四 header 与分页示例）。

## 6. 未完成事项（按优先级）

1. **打 tag 锁基线**：Unreleased 已积累大量 breaking（错误体系 FvError/
   命名对齐/日志完整工具），首个业务服务接入前建议打 v0.6.0 并全量验证
2. **实战缺口补齐**（security-management 检验发现，详见其 README）：
   - 事务 context 约定（conn 层 NewTxContext/FromTxContext，用户↔角色
     关联场景急需）
   - 通用仓储查询缺失（Repository[T] 只有 Create/Update，补 Get/List/
     Delete/Count 或文档化"查询走 GORM 出口"）
   - 配置加载器（env/file → Config）、审计内存场景 helper、
     NewPageable 空值类型推断
3. **snowflake WIP 修复**：测试 race（`nowFn` 改 atomic）+ 3 条 lint + 覆盖率
   补齐——修完 `make test/cover/lint` 三门槛即全部恢复
4. **repository 测试**：baseOf 校验可单测（参照 `base/audit_test.go` 的封印
   接口写法）；Create/Update 落库用 sqlmock（conn 包 `testutil_test.go` 有
   现成 connDialector 可参考）或进集成线
5. **txId 缺失行为拍板**：业务方定规则后改 `runtimecontext.go` 一处
6. **集成线补跑**：本轮 FvError/日志/i18n 改造后 Oracle/CH/OS 真环境链路
   建议跑 `make integration`（v0.5.1 时曾靠它抓出四处真库问题）
7. （远期）module 名迁移到真实托管路径：改 module 名 + 业务仓 `gofmt -r`/
   sed 批量替换 import + 删 replace，公共 API 零变化

## 7. 约定速记（评审必查）

- handler 出错：`c.Error(err)` + return，不写响应；未知错误根因只进日志
- 业务错误一律 `fverr.NewError(code, msg模板, args...)` + `InsertArgs`/
  `AttachError` 链；错误编码不出响应体；占位符统一 `{0}` 位置式（禁 fmt 动词）
- 模块错误语言包：`messages_<语言>.go` + `go generate`（cmd/i18nerr）；
  新增错误 = 改语言文件 + 重跑 generate + 用生成的构造函数
- 日志用 `logx.New(Config)` 组合出口；绝不在业务代码里手拼日志格式
- 出口对 FvError 按**自身 Code** 映射状态码，根因不参与（要 404 语义请
  自带 CodeNotFound，经 WithStatusMap 映射模块码）
- GORM 出口裸 `*gorm.DB`，链式调用记得 `.WithContext(ctx)`
- 事务内调存储过程：`conn.FromGorm(tx)` 取 Execer → `conn.CallProcTx`
- 新增跨层共享概念：在 `common/` 加零依赖小包并加进 depguard strict 白名单
- 依赖升级必须全量测试后单独提交并记 CHANGELOG；godror 处于 v0.x，
  升级必跑 Oracle 集成线
- 响应体/分页/上下文的任何字段调整都是 Breaking Change，记 CHANGELOG 并同步
  README 契约章节与 STATUS
