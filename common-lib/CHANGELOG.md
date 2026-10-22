# Changelog

本库遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 格式，
版本号遵循语义化版本。业务系统按 git tag 锁定版本，升级依赖必须全量测试
通过后单独提交并记录于此。

## [Unreleased]

### Added（readyz 就绪探活 handler + commonerrors 公共错误字典）

- 新增 `api/handlers` 包（公共 handler 集合，按 handler 一个文件追加）：
  `Readyz`（GET **/api/readyz**，server.NewEngine 内置注册、访问日志默认
  跳过）：恒返回无数据的 OK 响应体（`{"code":"OK",...,"data":null}`），
  真实依赖就绪判定后续在此扩展
- 新增 `common/commonerrors` 公共错误字典（中英文，**默认英文**；key 即
  响应体业务 code，fault 直接透传）：`ParameterRequired`/`InvalidParameter`/
  `InvalidTime`/`InternalError`；与模块 errors 同款 i18nerr 模式生成
  （Code 常量 + ErrorCode 字典句柄 + New 构造函数），业务服务报错时可直接
  引用组织错误（depguard 新增规则：仅依赖 i18n 与 bizerror）
- **Breaking**：`request.Require`/`request.Time` 签名增加首参
  `context.Context`（构造错误需按请求语言查公共字典）；错误从
  VALIDATION 硬编码英文改为公共错误码 ParameterRequired/InvalidTime
  （中英文按请求语言）
- api 层剩余硬编码错误文案随迁 commonerrors：`response.Fail` 空串兜底
  改公共错误 InternalError（按请求语言翻译，code 亦用 InternalError，
  调用方 WithCode 可覆盖）；`guard.Guard` panic 路径同样走 InternalError
  （原 code "FAIL"→"InternalError"）；fault 的非 BizError errMsg 直出
  err.Error() 原文行为不变（业务方拍板）

### Changed（Breaking：code 统一为字符串，与 bizerror.Code 同一命名空间）

- `ApiResponse.Code` int → **string**：`OK` 默认 **"OK"**、`Fail` 默认
  **"FAIL"**（常量 `response.CodeOK/CodeFail`），`WithCode(code string)`
- **fault 处理 BizError 时 code 直接透传**（`bizerror.Code` 字符串即响应
  体业务 code，如 "UserNotFound"）；非 BizError 默认 "FAIL"
- **WithCodeMap 机制删除**（字符串统一后映射无意义）：`fault.WithCodeMap`、
  `server.NewEngine` 的 codeMap 参数、SM 的 `usererrors.CodeMap()` 全部
  移除——模块码零注册零映射，响应体 code 即模块码
- i18nerr 生成器注释随迁（Code 常量即响应体业务 code），三处 errors_gen
  重新生成；wire_gen 随 NewEngine 无参化重生成

### Changed（Breaking：errorhandler 更名 fault 并简化职责）

- `api/middleware/errorhandler` → **`api/middleware/fault`**，
  `ErrorHandler()` → `Fault()`（统一错误出口）
- 职责简化为：采集 handler 链经 c.Error 上报的错误（不区分 BizError）→
  **记失败日志与调用栈**（bizCode/details 留痕）→ 经 `response.Fail` 写回。
  **errMsg 直取错误消息、出口零翻译**：BizError 取 Message，其余取
  err.Error() 原文直出响应体（原「未知错误根因不进响应」的脱敏行为废除，
  业务方拍板取舍）
- **删除哨兵翻译**：gorm.ErrRecordNotFound/sql.ErrNoRows → "resource not
  found"、context.Canceled → "client canceled request"、其余 → "internal
  error" 的固定文案替换全部移除
- **删除 WithMessageFunc**（errMsg 出口改写与直取语义矛盾）及
  server.NewEngine 的 WithMessageFunc 透传；**WithCodeMap 保留**（code
  语义不变）
- 注册顺序契约表述：observer.Observe() → fault.Fault() → guard.Guard()
  → enrich.Enrich()

### Changed（Breaking：四中间件按业务方拍板更名）

- `accesslog` → **`observer`**：`AccessLog()` → `Observe()`（访问观察者，
  记录访问日志）
- `recovery` → **`guard`**：`Recovery()` → `Guard()`（守护兜底，panic →
  500 统一体）
- `runtimecontext` → **`enrich`**：`FillRuntimeContext()` → `Enrich()`
  （上下文丰富，组装注入请求运行时上下文；五请求头常量随包迁移
  `enrich.HeaderTxID` 等）
- 注册顺序契约表述随迁：`observer.Observe() → errorhandler.ErrorHandler()
  → guard.Guard() → enrich.Enrich()`

### Changed（Breaking：请求运行时上下文收敛单路存储，api/reqctx 包删除）

- RuntimeContext 只存请求 context 一份：`FillRuntimeContext` 注入方式
  收敛为替换 `c.Request`（删除 gin.Context 侧 `c.Set` 的第二份存储），
  读取方一律 `rctx.GetRuntimeContext(c.Request.Context()).FromXxx()` 直取
  （响应体 txId、访问日志、审计钩子全链路同源；认证中间件更新操作者后
  不再有旧值漂移）
- **`api/reqctx` 包删除**：五请求头常量移入生产方
  `api/middleware/runtimecontext`（HeaderTxID 等）；`RuntimeContextOf/
  TxIDOf/LangOf` 语法糖删除（rctx 直取）；`WithActor` 删除（认证中间件
  按 HANDOVER 4.3 样例手写 `c.Request = c.Request.WithContext(
  rctx.SetRuntimeContext(...))` 一行）
- 迁移对照：`reqctx.TxIDOf(c)` → `rctx.GetRuntimeContext(c.Request.
  Context()).FromID()`；`reqctx.HeaderXxx` → `runtimecontext.HeaderXxx`；
  `reqctx.WithActor(c, id)` → 见 HANDOVER 4.3

### Changed（Breaking：统一响应体重塑为 ApiResponse）

- `response.Response` → `response.ApiResponse`，字段 `Contents` → **`Data`**
  （json tag "contents" → "data"）；data 由业务 handler 指定：single 对象
  或 multi 对象集合（分页 pageable 整体作为 data）
- 出口收敛为两个：`OK(c, data, opts...)`——code 默认 200、success=true、
  errMsg 默认空串（可 WithErrMsg）；`Fail(c, errMsg, opts...)`——**code 默认
  200**（可 WithCode 修改，HTTP 跟随）、success 恒 false、errMsg 必填（空串
  兜底 "internal error"）、data 恒 nil。**删除** Created/NoContent/OKPage/
  NewResponse（创建场景用 OK；204 场景改为 OK(c, nil)；分页 OK(c, pageable)）
- `errresp.Fail` 删除（被 response.Fail 取代），errresp 包收编为纯
  errorhandler 用的错误码 → HTTP 状态码映射；TxID 统一取 RuntimeContext
  记录的事务 ID
- 新增 VO → DTO 出参转换：`response.Wrap[DTO](vo)`（字段名 tag 优先忽略
  大小写，time → "yyyy-MM-dd HH:mm:ss" 字符串、bool → int 1/0、整数 →
  字符串；嵌套 struct/slice/map/指针递归；其余类型不匹配报错）；
  `common/units` 增 `FormatDateTime`（零值转空串）与 `BoolToInt`
- security-management 随迁：UserDTO.Enabled bool→int（入参 CreateUserCmd
  同步 int，app 层 units.Bool 转换）、handler 统一 OK、删除 204→200+data null

### Added（cmd/server 统一装配包，2026-10-21 拍板方案 A 落地）

- 新增 `cmd/server`（业务组装根的全家桶入口，import 别名 server）：
  - `NewEngine(statusMap, opts...)`：按固定顺序挂四中间件 + 内置 /healthz
    探活；模块错误码表是业务唯一必传差异；`EngineOption` 留
    WithSkipPaths/WithMessageFunc 逃生舱
  - `Init(m) (Config, error)`：组件自治读 server 段（段缺失全默认：
    addr ":8080"、mode release、readHeaderTimeout 5s、read/write 30s、
    idle 120s、shutdownTimeout 10s）；副作用设置 gin 模式（配置优先于
    GIN_MODE）；时长写 "5s" 式字符串
  - `NewServer(cfg, engine) (srv, cleanup)`：cleanup 承载优雅停机（等待
    在途请求，上限 ShutdownTimeout）；`Run(srv)`：阻塞运行，SIGINT/
    SIGTERM 后返回，生命周期日志走 logx 全局出口
  - `ProviderSet`（wire 编译期装配：配置 map、logx.Init、server.Init、
    NewServer；engine 归业务一行 NewEngine provider）；新增依赖
    google/wire
- `common/config` 新增 `Bind(m, out)`：绑定到已存在目标（保留已有值、
  出现的键覆盖——缺省即默认值的组件自治底座）；GetSection 与 Bind 均挂
  字符串时长钩子（"5s" → time.Duration）
- security-management 已迁移 wire 形态（wire.go/providers.go/瘦身 main，
  cleanup 链逆序：HTTP 停机 → 关存储 → 关日志）；application.yml 增
  server 段；usererrors.StatusMap() 码表单一来源；apifull 改为无 wire 的
  手工全家桶示例

### Changed（Breaking：中间件零持有日志）

- `accesslog.AccessLog` / `errorhandler.ErrorHandler` / `recovery.Recovery`
  去掉首参 logx.Logger，内部直接经 logx 全局出口记录（与拍板的日志哲学
  一致；消除「构造期捕获 Default() 早于 logx.Init 永远 Nop」的时序隐患；
  测试经 logx.SetLogger 注入，testutil 新增 CaptureLog 助手）
- 迁移对照：`AccessLog(log)` → `AccessLog()`、`ErrorHandler(log, opts...)`
  → `ErrorHandler(opts...)`、`Recovery(log)` → `Recovery()`；业务组装
  建议直接改用 `server.NewEngine`

### Added（common/units 输入转换原语 + api/request 参数转换）

- 新增 `common/units`（跨层零依赖）：`Bool`（布尔 wire 约定：int 输入
  **1 → true，其余含 0 → false**，零值即默认 false）、`ParseDate` /
  `ParseDateTime`（`yyyyMMdd` / `yyyyMMddHHmmss` 字符串 → **本地时间**，
  布局常量 `LayoutDate/LayoutDateTime` 导出）、`ParseEpochMillis`
  （13 位毫秒时间戳字符串 → 本地时间）
- `api/request` 新增：`Time(name, v)`——时间字符串参数解析（8 位 yyyyMMdd /
  14 位 yyyyMMddHHmmss / 13 位毫秒按长度识别，空串按可选参数返回零值，
  非法报 VALIDATION 带参数名）；`Wrap[VO]` 的时间钩子——DTO 时间字符串
  字段自动解析为 VO 的 time.Time 字段（同格式识别，空串产出零值）；
  `Wrap[VO](src)`——api 层 DTO → service 层 VO，
  字段名忽略大小写对齐自动拷贝 + WeaklyTypedInput 弱类型转换（int → bool
  经 DecodeHook 按 units.Bool 约定，非 1 一律 false；需要组装裁剪的场景
  业务手写转换）；`Require(name, v)`——必填字符串校验，去首尾空白，
  空则返回 VALIDATION 错误（errMsg 携带参数名）
- `api/middleware/runtimecontext` 的 txTime 解析改用 `units.ParseDateTime`：
  **时区行为变化**——原按 UTC 解析，现按服务器本地时区解析（wall-clock
  字符串语义更正确）

### Changed（Breaking：API 层按功能分包）

- `api` 根包删除，按功能拆为四包：`api/reqctx`（gin ↔ rctx 绑定：五请求头
  常量、CtxRuntimeContext key、RuntimeContextOf/TxIDOf/LangOf 取值语法糖、
  WithActor）、`api/request`（请求参数对象解析，PageQuery）、`api/response`
  （统一响应体 apiresponse：Response/OK/Created/NoContent/OKPage/Write）、
  `api/errresp`（错误响应：Fail、DefaultStatusMap/StatusFor、
  StatusClientClosedRequest）
- `api/middleware` 按处理流程点拆为四子包：`accesslog` / `errorhandler` /
  `recovery` / `runtimecontext`；根包保留为注册顺序契约（纯文档）与跨四包
  全栈集成测试。`accesslog.AccessLogOption` 更名 `Option`（分包后不再口吃）
- 新增 `api/internal/testutil`（api 子树共享测试脚手架：日志捕获/测试引擎/
  请求发起/kv 断言，internal 保证业务不可见）
- 迁移对照：`api.OK/OKPage/Created/NoContent` → `response.*`；`api.Fail`/
  `api.StatusFor`/`api.DefaultStatusMap` → `errresp.*`；`api.PageQuery` →
  `request.PageQuery`；`api.HeaderXxx`/`api.CtxRuntimeContext`/
  `api.RuntimeContextOf`/`api.TxIDOf`/`api.LangOf`/`api.WithActor` →
  `reqctx.*`；`middleware.AccessLog/ErrorHandler/Recovery/FillRuntimeContext`
  → `accesslog./errorhandler./recovery./runtimecontext.` 同名函数；
  `middleware.WithStatusMap/WithMessageFunc` → `errorhandler.*`；
  `middleware.WithSkipPaths` → `accesslog.WithSkipPaths`。函数签名与行为
  零变化（纯结构重构）；examples/apifull 与 security-management 已随迁

### Changed（Breaking：分页对象更名与出参补 totalPage）

- `paging.Query` → `paging.PageRequest`（请求对象贯穿 api/service/storage，
  命名对齐"请求"语义）；默认值调整：pageSize 默认 **10**（原 20）、
  pageIndex 默认 1 不变；每页硬上限更名 `MaxPageSize` 且降为 **100**
  （原 MaxSize 500）
- `paging.Pageable` 出参新增 **totalPage** 字段（totalSize/pageSize 向上
  取整：不整除进一、整除不加，totalSize=0 时为 0）；字段顺序对齐
  pageIndex/pageSize/totalSize/totalPage/data——统一响应体 contents 的
  分页 wire 格式随之变化（Breaking）
- 迁移对照：`paging.Query` → `paging.PageRequest`、`paging.MaxSize` →
  `paging.MaxPageSize`；`Normalize/Limit/Offset/NewPageable` 签名不变；
  api.PageQuery、opensearch.search.WithPaging 随迁

### Changed（Breaking：actortx 并入 rctx 与 RuntimeContext 私有化）

- `common/actortx` 包删除：操作者（Actor）本质即 RuntimeContext.User，
  原双通道（FillRuntimeContext 里 userName 同时注入 rctx 与 actortx）
  合并为一条——oracle BasePO 审计钩子直接从
  `rctx.GetRuntimeContext(ctx).FromUser()` 取值；`api.WithActor` 内部改
  经 rctx 派生 WithUser
- `RuntimeContext` 字段全部私有：`WithID/WithTime/WithName/WithUser/
  WithLang` 链式赋值（值语义副本，同 Config.WithDefaults 模式）、
  `FromID/FromTime/FromName/FromUser/FromLang` 取值；
  SetRuntimeContext/GetRuntimeContext 不变
- 迁移对照：`actortx.SetActorContext(ctx, id)` →
  `rctx.SetRuntimeContext(ctx, rctx.RuntimeContext{}.WithUser(id))`；
  `rc.ID` 等字段访问 → `rc.FromID()` 等

### Changed（Breaking：统一错误更名 BizError）

- `common/fverr` 更名为 `common/bizerror`，类型 `FvError` → `BizError`
  （2026-10-21 拍板：体系本身不变，仅更名）；`Code`/`NewError`/`InsertArgs`/
  `AttachError`/`IsError`/`From` 等符号不变，业务方批量替换 import 路径与
  类型名即可（`gofmt -r` 或 sed 一次跑完）；cmd/i18nerr 生成器同步更名，
  已有模块重跑 `go generate` 零漂移（examples 与 security-management
  已随迁）

### Changed（Breaking：日志工具重构为配置驱动）

- `common/logx` 重构为**组件自治 + 配置驱动**（2026-10-21 拍板）：公开 API 收敛
  为 `Init(m map[string]any) (Logger, func(), error)`——读取配置 log 段（配置
  结构体全部私有，不再对外暴露），成功后自动落位全局出口；新增包级记录函数
  `Debug/Info/Warn/Error(msg, kv...)`，业务零持有日志对象；`SetDefault` 更名
  `SetLogger`（测试注入与 Init 落位共用）
- **出口收敛为 console/file 两种**：每出口 `enabled/level/format`（format 取
  json（默认）|console，非法取值静默按 json）；等级链：出口 level →
  log.level → info 静默兜底；两个出口均未启用（或缺失 log 段）返回
  `ErrNoLogComponent`
- **文件滚动从自研编号方案（log、log.1 ... log.N）换为 lumberjack**
  （推翻 2026-10-20 编号滚动拍板）：新增 `maxAge`（按天清理备份）、
  `compress`（旧备份 gzip）、`localTime`（备份文件名时间戳时区，默认 true）；
  默认值 path=systemlogs/server.log、maxSize=100（兆）、maxBackups=100、
  maxAge=30、compress=false；配置 key 首字母小写 camelCase
- **远程出口整体移除**：`Sink` 接口、异步批量缓冲 core、`RemoteEncoder`、
  `storage/opensearch/logsink` 包删除；自定义格式注入机制（`Encoder` 字段、
  `LineEncoder`、`Formatter`、`Rec`）随 format 预置化一并删除；历史 slog
  适配器 `NewSlog`（v0.1.0 最小接口时代产物）一并删除，示例 oracleproc 改用
  `logx.Init`
- 日志行携带 **caller 调用位置**：`AddCaller + AddCallerSkip(1)` 跳过
  logx 包装层；包级函数经 zap 实例直调（不经接口中转），caller 指向业务
  调用行；`SetLogger` 注入自定义实现时包级函数自动降级走接口
- Init 内用 `common/config` 的 `ValidateIsBoolean/IsNumber` 对存在的键做断言式
  校验（宽松规则，环境变量覆盖产生的字符串放行）、`GetSection` 绑定私有配置
- 新增依赖 `gopkg.in/natefinch/lumberjack.v2`（depguard 白名单固化）；
  迁移对照：`logx.New(Config{...})` → `logx.Init(cfgMap)`（application.yml 的
  log 段）、`logx.SetDefault` → `logx.SetLogger`；security-management 已随迁
  （新增根目录 application.yml，cmd/server 经 `config.LoadConfig("security")`
  + `logx.Init` 装配，端到端实测通过）

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

- 配置加载能力 `common/config`（2026-10-21 拍板：每业务服务自维护
  application.yml，库只提供加载/读取/校验能力，不内置任何配置项）：
  - `LoadConfig(appName)`：文件路径取环境变量 `{appName}_CFG`（缺省为工作
    目录下 application.yml）；仅支持 yml/yaml/json，其余报 `ErrInvalidConfigFile`；
    解析为嵌套 `map[string]any` 后逐叶子应用环境变量覆盖（叶子路径点连接加
    appName 前缀，如 `security.log.level` 覆盖 log.level），未设置保留原值，
    覆盖值为字符串原样进 map、类型转换推迟到绑定阶段
  - `GetSection[T](m, section)`：点分下钻取子 map 绑定到目标结构体
    （mapstructure tag 优先，无 tag 字段按完整字段名忽略大小写；开启
    WeaklyTypedInput），未命中返回 `ErrSectionNotFound`；`Get(m, path)`
    返回原始子 map，未命中为 nil 不报错
  - `ValidateIsEmpty/IsNumber/IsBoolean(m, key)`：断言式校验，宽松规则与
    绑定一致（字符串 "200" 算数字、"true" 算布尔，与环境变量覆盖机制兼容），
    分别返回 `ErrValueEmpty`/`ErrInvalidNumberValue`/`ErrInvalidBoolValue`
  - 新增依赖 `go-viper/mapstructure/v2`（depguard 白名单固化）；
    `go.yaml.in/yaml/v3` 由 indirect 转直接依赖
- 日志升级为完整工具：`common/logx` 引擎改为 **zap**（业界事实标准），
  `logx.New(Config)` 出口可组合——标准输出 / 文件（**自研编号滚动**：
  log、log.1、log.2 ... log.N 按大小触发，保留 N 份）/ 远程批量出口
  （异步，默认 256 行/批或 5s 刷出，缓冲满丢弃计数绝不反压业务）；**每个
  出口可注入自定义编码器**（`Encoder` 字段 + `logx.LineEncoder` 适配业务
  格式函数，文件字符串格式与远程 JSON 可各配各的）；新增
  `storage/opensearch/logsink` 实现 logx.Sink（Bulk API 直写按日索引
  `logs-yyyy.MM.dd`）。Logger 五方法接口保留为库内组件与业务的接缝
  （`NewSlog` 适配 slog 兼容旧接入）；新增依赖 `go.uber.org/zap`
  （depguard 白名单固化）
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
