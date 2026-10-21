# 交接文档（HANDOVER）

> 面向接手本库的工程师：项目是什么、怎么组织的、核心契约长什么样、
> 怎么开发验证、还欠什么。规范细节以 [README](README.md) 为准，
> 实时状态与待办见 [STATUS](STATUS.md)，变更史见 [CHANGELOG](CHANGELOG.md)。

## 1. 项目是什么

fab-viewer 体系的 Go 公共库（module：`github.com/fab-viewer/common-lib`，
module 名为占位、业务方经 `replace` 本地引用；2026-10-20 起 git 根上移至
上级 `services/`，本库为其中的 `common-lib/` 子目录，与业务服务
security-management 并列，仓库级交接见上级 [HANDOVER](../HANDOVER.md)）。
被两类业务系统 import：

- **RESTful API 业务**（Gin）：用 `api/*`（reqctx/request/response/errresp 与
  `api/middleware/*` 四子包）+ 存储包
- **内部定时器业务**（无接口层）：只用跨层包 + 存储包

硬约束：**零后台 goroutine**、全同步调用路径、import 路径即能力边界。

## 2. 核心设计理念（务必先懂再动）

1. **import 路径即能力边界**：`api/*` 各包只准接口访问层 import
   （Gin 依赖从这里开始）；`storage/*` 只准存储层 import；`common/*` 跨层通用。
   由 `.golangci.yml` depguard 强制（deny 是前缀匹配，能命中全部子包）。
2. **跨层会合点**：`api` 与 `storage` 互不感知，只在小包相会——
   `bizerror`（统一业务错误 BizError）、`paging`（分页对象）、`rctx`
   （运行时上下文，含审计操作者）。需要两边共享的概念，加包而不是加依赖。
3. **DTO/VO/PO 对象流转**：DTO 只在 api 层进出；VO 是 storage 对上的唯一边界
   类型（禁止 gorm/ch tag）；PO 未导出锁死在业务存储包内（编译器强制）。
   详见 README「对象层约定」。
4. **配置与装配分离**：`*/config` 子包是纯数据+校验；SDK Options/DSN 之类
   的装配逻辑在连接包。**业务服务自维护 application.yml**，加载/读取/校验
   能力在 `common/config`（只提供能力，不内置任何配置项）。
5. **组件自治**（2026-10-21 拍板，进行中）：每个组件（logx 已完成、存储
   包待做）自己提供「从 config map 初始化」的方法（`Init(m)`），是否启用
   由组件按自己的配置段判断（enabled 开关或段缺失，组件自己定）；
   `cmd/server` 只负责依次调用（统一编排入口，与 wire 编译期装配，
   详见第 6 节）。
6. **逃生舱模式**：包装层不遮蔽 SDK——`clickhouse.Conn()/NewFromConn/WrapDB`、
   `opensearch.SDK()/NewFromAPI/DoSearch`、`oracle DB.GORM()/SQL()`。

## 3. 包地图

```
common/config     配置加载：LoadConfig（{appName}_CFG→application.yml，yml/yaml/json，
                  叶子环境变量覆盖 security.log.level 式）、GetSection[T]（mapstructure
                  绑定）、Get、ValidateIsEmpty/IsNumber/IsBoolean（宽松判断）
common/bizerror   BizError（唯一构造 NewError(code,msg,args...)，链式 InsertArgs/AttachError，
                  IsError/From）；原 fverr 更名（2026-10-21）
common/paging     PageRequest{PageIndex,PageSize}（请求贯穿全链路；默认 1/10、
                  上限 MaxPageSize=100）、Pageable[T]（出参，含 totalPage=ceil）
common/logx       日志完整工具：Init(m) 唯一初始化（配置结构私有）+ 包级
                  Debug/Info/Warn/Error + SetLogger；zap 引擎，console/file
                  双出口（lumberjack 滚动），caller 指向业务调用行
common/rctx       RuntimeContext（五字段私有，WithXxx 链式赋值/FromXxx 取值；
                  User 即审计操作者——原 actortx 包已并入删除）
common/i18n       Bundles 模块多语言表 + Accept-Language 解析（只提供能力，
                  译文业务每模块自维护；仅依赖 rctx）
api/reqctx        gin ↔ rctx 绑定：五请求头常量、CtxRuntimeContext key、
                  RuntimeContextOf/TxIDOf/LangOf 取值语法糖、WithActor
api/request       请求参数对象解析：PageQuery（pageIndex/pageSize → PageRequest）
api/response      统一响应体 apiresponse：Response 五字段恒序列化 +
                  OK/Created/NoContent/OKPage/Write 写回
api/errresp       错误响应：Fail、DefaultStatusMap/StatusFor（Code → HTTP 状态码）
api/middleware    按处理流程点分包（根包仅承载注册顺序契约与全栈测试）：
                  accesslog → errorhandler → recovery → runtimecontext（顺序固定）
storage/oracle/conn        Open、DB 三通道（GORM/SQL/存储过程）、Cursor、driverAdapter
storage/oracle/config      Config/WithDefaults/Validate/BuildDSN
storage/oracle/base        BasePO 审计基类（GORM 钩子自动填 CreatedBy 等，取 rctx.User）
storage/oracle/repository  Repository[T]（雪花 ID + 审计落库）
storage/clickhouse(+config/batch)   Open/Client/QuerySlice；Batch[T] 批量写
storage/opensearch(+config/search)  Open/Client/Index/Get/Delete；Search[T]（logsink 已删）
storage/snowflake  点分隔雪花 ID（"<Unix毫秒>.<节点>.<序列>"；WIP，见 STATUS）
storage/pool.go    PoolConfig 连接池配置
cmd/i18nerr        模块错误字典生成器（messages_<语言>.go → errors_gen.go）
examples/          apifull（含 i18n 完整示例）/ oracleproc / chbatch / ossearch
```

依赖方向（无环）：config ← conn ← repository；base 只依赖 rctx+gorm；
logx 依赖 config（段读取）；batch/search 依赖各自根包；所有存储子包互不
依赖（depguard 强制）。

## 4. 核心对外契约（wire 格式）

### 4.1 统一响应 apiresponse（五字段恒序列化，成功/失败同构）

```json
{"code":200,"success":true,"errMsg":"","contents":{...},"txId":"abc"}
{"code":404,"success":false,"errMsg":"user 999 not found","contents":null,"txId":"abc"}
```

- `code` = HTTP 状态码（数字），由 `bizerror.Code → DefaultStatusMap` 映射产生
- 字符串错误码与 Details **不出参**，随 `business error` 日志留痕
- 业务错误统一走 `c.Error(err)` → ErrorHandler，handler 不自写错误响应

### 4.2 分页（2026-10-21 拍板新契约）

- 入参：`?pageIndex=2&pageSize=10`（`request.PageQuery` 解析为
  `paging.PageRequest`；非法回退 **1/10**，pageSize 上限 **MaxPageSize=100**）
- 出参（置于 contents）：`{"pageIndex":2,"pageSize":10,"totalSize":57,
  "totalPage":6,"data":[...]}`——totalPage 为 totalSize/pageSize **向上取整**
  （不整除进一、整除不加，totalSize=0 为 0）

### 4.3 请求运行时上下文（rctx）

请求头四字段 + `Accept-Language` → `runtimecontext.FillRuntimeContext` →
`rctx.RuntimeContext`（**字段私有**）两路注入（请求 context + gin）：

| header | 赋值 | 取值 | 兜底 |
|---|---|---|---|
| txId | WithID | FromID | 缺失生成 32 hex（**行为待定**，改 `runtimecontext.go` 一处） |
| txTime | WithTime | FromTime | yyyyMMDDHHmmss；缺失/非法→服务器时间 |
| txName | WithName | FromName | 可空 |
| userName | WithUser | FromUser | 可空；**即审计操作者**（BasePO 钩子自动填） |
| Accept-Language | WithLang | FromLang | q 值降序取首选并规范化；缺失/非法为空串 |

业务层读取：`rctx.GetRuntimeContext(c.Request.Context()).FromUser()`。
定时任务入口自注入：`rctx.SetRuntimeContext(ctx, rctx.RuntimeContext{}.
WithID(id).WithName("job"))`。

### 4.4 错误消息国际化

`common/i18n` 只提供能力、译文由业务服务自注册；errMsg 按请求语言翻译：

- 接入（每模块自己的多语言表，编译期进包、无全局注册）：模块内
  `messages_<语言>.go`（key 即 code，{0} {1} 位置占位符）→
  `cmd/i18nerr -dir . -pkg xxx -default en` 生成 `errors_gen.go`（Code 常量、
  ErrorCode 字典句柄、构造函数）→ 调用处 `xxx.NewUserNotFound(ctx).
  InsertArgs(id).AttachError(root, details)`——构造函数内部按请求语言查模板后交给
  `bizerror.NewError`；出口直出 Message 不翻译
- 回退链：请求语言 → 基础语言（只收窄不放宽）→ 默认语言 → key 原文
- 删除确认等带实体名的消息：模板 `{0}` + `InsertArgs(name)`；模块码经
  `WithStatusMap` 映射状态码

### 4.5 日志（logx，2026-10-21 拍板重构）

- **唯一初始化入口 `logx.Init(cfgMap)`**（由 cmd/server 统一装配调用）：
  读 log 段（配置结构私有）→ 成功后自动落位全局出口 → 业务用包级函数
  `logx.Info(msg, kv...)` 直接记录，零持有日志对象；测试经 `SetLogger`
  注入（原 SetDefault 更名）
- 双出口 `console`/`file`，各自 `enabled/level/format`；等级链：出口 level →
  log.level → info **静默兜底**；format：`json`（默认）|`console`
- 文件滚动 **lumberjack**（推翻原自研编号滚动）：path（默认
  systemlogs/server.log）、maxSize=100（兆）、maxBackups=100、maxAge=30（天，
  按天清理）、compress=false（旧备份 gzip）、localTime=true（备份名时区）
- 两个出口均未启用 → `ErrNoLogComponent`
- caller 经 `AddCallerSkip(1)`+包级函数直调 zap 指向**业务调用行**
- 远程出口（logsink/异步缓冲）与自定义格式机制（LineEncoder）已删除

### 4.6 配置文件（common/config，2026-10-21 新增）

- 每业务服务自维护 `application.yml`（路径可经 `{appName}_CFG` 环境变量
  覆盖；仅 yml/yaml/json）
- 叶子环境变量覆盖：路径点连接加 appName 前缀，如 `security.log.file.maxSize=200`
  覆盖 `log.file.maxSize`；覆盖值为字符串原样进 map，绑定/校验阶段按宽松
  规则转换（"200" 算数字、"true" 算布尔）
- `GetSection[T](m, "log.file")` 绑定到目标结构体（mapstructure tag 优先，
  无 tag 按完整字段名忽略大小写）

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
含四 header 与分页示例）。业务服务装配自证：security-management 的
`go run ./cmd/server`（memory 模式自举，读 application.yml）。

## 6. 进行中：cmd/server 统一装配 + wire（2026-10-21 拍板，未落地）

- **全家桶 + 组件自治**：`cmd/server` 包静态 import 全部组件，CreateServer
  依次调用各组件的 `Init(m)`；是否加载由组件自己的配置段决定（组件自定
  enabled 或段缺失判断）
- **依赖注入选 wire（编译期生成，方案 A）**：排除 uber/fx/dig（运行时反射
  + goroutine，违反零后台 goroutine 约束）。common-lib 只导出
  `server.ProviderSet`（cmd/server/set.go，库 API 的一部分，本库自己无
  wire.go 不跑 generate）；业务服务建仓时在自己的 cmd/<svc>/wire.go 里
  引用一行 `server.ProviderSet`；依赖图变了才重跑 wire，忘了重跑在编译期
  暴露。wire cleanup 机制（provider 返回 `(T, func(), error)`）承载优雅停机
- logx 已按此形态完成（`Init(m) (Logger, func(), error)`）；存储组件的
  Init 形态、四中间件注册方式、server 段（addr 等）**待设计拍板后落地**

## 7. 未完成事项（按优先级）

1. **cmd/server + wire 落地**（第 6 节）：待四中间件、server 段、存储组件
   Init 形态设计讲完
2. **实战缺口补齐**（security-management 检验发现）：
   - 事务 context 约定（conn 层 NewTxContext/FromTxContext，用户↔角色
     关联场景急需）
   - 通用仓储查询缺失（Repository[T] 只有 Create/Update）
   - 审计内存场景 helper、`NewPageable` 空值类型推断（差异点 #4/#5）
3. **snowflake WIP 修复**：测试 race（nowFn 改 atomic）+ 3 条 lint + 覆盖率
   + **同毫秒序列缺陷**（同毫秒内序列恒为 0 → ID 重复；security-management
   的 app 测试间歇失败实证，见 STATUS）
4. **repository 测试**：baseOf 校验可单测；Create/Update 落库用 sqlmock 或
   进集成线
5. **txId 缺失行为拍板**：业务方定规则后改 `runtimecontext.go` 一处
6. **集成线补跑**：本轮 config/logx/bizerror/rctx/paging 大改后 Oracle/CH/OS
   真环境链路建议跑 `make integration`
7. **打 tag v0.6.0 锁基线**：Unreleased 已积累多轮 breaking，首个新业务
   接入前建议打 tag 并全量验证
8. （远期）module 名迁移到真实托管路径：改 module 名 + 业务仓批量替换
   import + 删 replace，公共 API 零变化

## 8. 约定速记（评审必查）

- handler 出错：`c.Error(err)` + return，不写响应；未知错误根因只进日志
- 业务错误一律 `bizerror.NewError(code, msg模板, args...)` + `InsertArgs`/
  `AttachError` 链；错误编码不出响应体；占位符统一 `{0}` 位置式（禁 fmt 动词）
- 模块错误语言包：`messages_<语言>.go` + `go generate`（cmd/i18nerr）；
  新增错误 = 改语言文件 + 重跑 generate + 用生成的构造函数
- 日志一律 `logx.Info/Warn/Error/Debug` 包级函数，禁止业务自建日志对象；
  携带固定字段用 `logx.Default().With(...)`
- 配置只写 application.yml（+环境变量覆盖），不写代码里；新增组件配置 =
  组件提供私有配置结构 + `Init(m)` + GetSection 绑定 + 存在才校验
- 出口对 BizError 按**自身 Code** 映射状态码，根因不参与（要 404 语义请
  自带 CodeNotFound，经 WithStatusMap 映射模块码）
- GORM 出口裸 `*gorm.DB`，链式调用记得 `.WithContext(ctx)`
- 事务内调存储过程：`conn.FromGorm(tx)` 取 Execer → `conn.CallProcTx`
- 新增跨层共享概念：在 `common/` 加零依赖小包并加进 depguard strict 白名单
- 依赖升级必须全量测试后单独提交并记 CHANGELOG；godror 处于 v0.x，
  升级必跑 Oracle 集成线
- 响应体/分页/上下文的任何字段调整都是 Breaking Change，记 CHANGELOG 并同步
  README 契约章节与 STATUS
