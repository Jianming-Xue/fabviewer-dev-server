# common-lib

fab-viewer 体系的 Go 公共库：集中提供**存储访问**（Oracle / OpenSearch）
与 **API 公共组件**（统一响应、分页、错误处理、日志中间件），被两类业务系统 import：

- **RESTful API 业务**（Gin）
- **内部定时器业务**（无接口访问层）

配套文档：[STATUS](STATUS.md)（当前状态与待办快照）、[HANDOVER](HANDOVER.md)
（交接说明：设计理念、包地图、契约速查、未完成事项）。

## 版本与环境要求

| 项 | 要求 |
|---|---|
| Go 工具链 | **≥ 1.26**（受 opensearch-go/v4 最低版本约束） |
| Oracle 访问 | 仅当 import `storage/oracle` 时：编译期 CGO + C 编译器；运行期 Oracle Instant Client |
| OpenSearch | 无额外系统依赖（opensearch HTTP） |

## 包 ↔ 业务分层映射矩阵（核心）

业务系统分层：接口访问层 / 服务层 / 业务聚合层 / 存储层 / 对象层（定时器业务无接口访问层）。
**import 路径即能力边界**：

| common-lib 包 | 接口访问层 | 服务层 | 业务聚合层 | 存储层 | 对象层 | main 组装根 |
|---|:---:|:---:|:---:|:---:|:---:|:---:|
| `common/bizerror` / `common/paging` / `common/rctx`（跨层，零三方依赖） | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `common/config`（配置加载：application.yml + 环境变量覆盖 + 段绑定） | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `common/logx`（日志完整工具：Init + 包级函数，见下节） | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `common/i18n`（多语言表能力，仅依赖标准库与 `common/rctx`）、`common/commoncodes`（公共码字典：中英文、默认英文，key 即响应体业务 code） | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `common/units`（输入转换原语：int 布尔、日期/毫秒时间戳解析，零三方依赖） | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `api/request`（请求参数解析/校验/时间解析/DTO 包装）、`api/response`（统一响应体 ApiResponse 与 VO→DTO 转换）、`api/handlers`（公共 handler：readyz 就绪探活）、`api/middleware/*`（observer / fault / guard / enrich 四子包） | ✓ | ✗ | ✗ | ✗ | ✗ | ✓ |
| `cmd/server`（组装全家桶：NewEngine 引擎装配 / Init 读 server 段 / NewServer+Run 优雅停机 / wire ProviderSet） | ✓ | ✗ | ✗ | ✗ | ✗ | ✓ |
| `storage/oracle`（`conn`/`config`/`base`/`repository` 四子包）/ `storage/opensearch`（`config`/`search` 子包）/ `storage/snowflake`（ID 生成） | ✗ | ✗ | ✗ | ✓ | ✗ | ✓ |

约定：

- **组装根豁免**：业务 `main` 不属于五层，可同时 import 两边（如 `/healthz` 由 main
  把存储层的 `Ping` 实现注入路由）。
- **SDK 类型不越层**：`*gorm.DB`、`*opensearch.Client` 等只允许出现在业务存储层内部，
  存储层对上的接口签名用业务自定义类型 + `errs`/`paging` 类型。
- **跨层共享靠依赖倒置**：接口层 `request.PageQuery(c)` 产出 `paging.PageRequest` 沿层传递，
  存储层消费 `q.Limit()/q.Offset()` 或 `search.WithPaging(q)`；`api` 与 `storage`
  互不感知，只在 `paging`/`errs` 相会。

## 对象层约定（DTO/VO/PO）——所有业务系统强制遵循

任何业务系统的**开发与新增功能**都遵循同一套对象流转方案，目标：**隐藏存储实现细节**，
让上层永远不感知 PO/表结构/存储介质的存在。

```
HTTP 请求                                          定时任务（无 DTO）
   │                                                    │
   ▼                                                    ▼
api 层          DTO 进出（json tag）              service / application 层
   │                                                    │
   ▼                                                    ▼
service / application 层  DTO → VO（组装/裁剪业务对象）
   │
   ▼
storage 层      对外签名只见 VO：
   写：VO → PO（包内转换）→ GORM / 原生协议
   读：PO / 行集 → VO 返回
```

三类对象的定义位置与规则：

| 对象 | 定义位置 | tag | 规则 |
|---|---|---|---|
| **DTO** | 对象层共享包 | 仅 json | 对外 HTTP 契约，只在 api 层进出；定时任务没有 DTO |
| **VO** | 对象层共享包 | 仅 json | 业务对象，storage 对上的唯一边界类型；**禁止携带任何存储 tag** |
| **PO** | **存储包内部、类型名未导出** | gorm | 编译器保证 PO 出现在导出签名即编译失败——隐藏存储细节由语言强制，不靠约定 |

写路径示例（PO 未导出，锁死在存储包内；配合库内置 upperNamer，PO 连 column tag 都可省）：

```go
// internal/storage/user_repo.go —— 包 internal/storage
type userPO struct { // 小写：编译器禁止它出现在任何导出签名
    ID   int64
    Name string
}

func (r *UserRepo) Create(ctx context.Context, vo model.User) error {
    po := userPO{ID: vo.ID, Name: vo.Name} // VO→PO 转换只发生在这里
    return r.db.GORM().WithContext(ctx).Create(&po).Error
}
```

读路径与原生 SQL 直出 VO 的标准姿势：

```go
// ORM 查询：PO→VO 后返回
var po userPO
db.GORM().WithContext(ctx).Where("ID = ?", id).First(&po)
return model.User{ID: po.ID, Name: po.Name}, nil

// Oracle 原生 SQL 直出 VO：GORM Scan 按列名映射，不需要 gorm tag
var vo model.User
db.GORM().WithContext(ctx).Raw("SELECT id, name FROM users WHERE id = :1", id).Scan(&vo)

// OpenSearch 直出 VO：_source 走 encoding/json，VO 已有的 json tag 直接复用
res, err := opensearch.Search[model.User](ctx, cli, "users", q)
```

**禁止事项**（评审必查）：

1. PO（或任何 gorm tag 结构）出现在导出签名、返回值、对象层共享包
2. VO 携带 gorm tag；DTO 出现在 storage 层
3. `*gorm.DB`、`driver.Rows`、`*opensearch.Client` 等 SDK 类型越出存储层

## 定时器业务（无接口访问层）的适配清单

内部定时任务没有 api 层，按以下清单裁剪（其余约定与 REST 业务完全一致）：

| 项 | 调整 |
|---|---|
| 分层结构 | 去掉接口访问层与 DTO；保留 service / application / storage / 对象层；main 兼任调度入口（组装根豁免仍适用） |
| 对象流转 | 简化为 **VO ↔ PO**（DTO→VO 一步消失，任务参数直接是业务参数） |
| 错误处理 | 没有 fault 中间件兜底——任务循环内自行处理：`logx.Error` 记录（bizerror 包装自带操作上下文），并按 `bizerror.Code` 决定重试/跳过/告警（如 CodeDB 重试、CodeValidation 告警并跳过） |
| 日志 | 装配时 `logx.Init(cfgMap)`；每个任务用 `logx.Default().With("job", "xxx_settle")` 派生，日志自动带任务标识 |
| 分页 | 批处理通常不翻页 |
| 事务 | 结算/对账类任务用 `GORM().Transaction` + `conn.CallProcTx`（conn.FromGorm 取事务连接），已有完整约定 |
| 优雅退出 | 信号 → 停止调度（不取新任务）→ 等待在跑任务完成（带超时）→ 依次 Close 各存储客户端 |
| depguard | 删除业务模板中 ifaces 相关规则（目录不存在自然不生效），其余规则不变 |
| 调度器 | 不在 common-lib 范围（本库不绑 cron 选型）；若未来多个定时系统需要任务锁/错过补偿等公共能力，再评估入库 |

## 快速开始

业务系统 `go.mod`（正式托管前的本地引用方式）：

```go
module your-business-system

require github.com/fab-viewer/common-lib v0.0.0

replace github.com/fab-viewer/common-lib => ../../services/common-lib
```

### RESTful API 业务（接口访问层 + 存储层）

引擎装配（四中间件固定顺序 + /healthz 探活）与 http.Server/优雅停机归
`cmd/server`，业务只传模块错误码表并注册路由：

```go
r := server.NewEngine() // 模块错误码（bizerror.Code）由 fault 直接透传响应体 code

r.GET("/api/users", func(c *gin.Context) {
    q := request.PageQuery(c) // 解析 ?pageIndex=&pageSize=，非法值回退默认
    res, err := userRepo.List(c.Request.Context(), q) // 返回 paging.Pageable[User]
    if err != nil {
        _ = c.Error(err) // 交给 fault 统一写回，不要自行写错误响应
        return
    }
    response.OKPage(c, res)
})

cfg, err := server.Init(cfgMap)   // 读 server 段（段缺失全默认），设置 gin 模式
srv, cleanup := server.NewServer(cfg, r)
err = server.Run(srv)             // 阻塞运行，SIGINT/SIGTERM 后返回；cleanup 优雅停机
```

手工挂中间件（不经 NewEngine）时注册顺序固定 `observer.Observe() →
fault.Fault(opts...) → guard.Guard() →
enrich.Enrich()`（Observe 必须在 fault 外层，
否则错误路径记录的 status 不正确；中间件零持有日志对象，经 logx 全局出口记录）。
wire 装配见 `cmd/server` 包文档与 security-management 的 cmd/server。

### 定时器业务（无接口访问层，只 import 存储包 + 跨层包）

完整适配清单见上文「定时器业务（无接口访问层）的适配清单」。

```go
db, err := conn.Open(ctx, config.Config{
    User: "app", Password: "***", Host: "db.example.com", ServiceName: "ORCLPDB1",
})
if err != nil { ... }

var n int64
err = db.CallProc(ctx, "P_ACCRUE", jobID, conn.Out(&n))
// 原生 SQL：rows, err := db.SQL().QueryContext(ctx, "SELECT ...")
// ORM：    db.GORM().WithContext(ctx).Find(&orders)   // 记得带 WithContext
```

## 依赖边界声明（谁会被 godror 影响）

Go 按 **import 的 package** 编译链接，不按 module：

- 只 import `api`（+跨层包）的业务：**不链接 godror**，`CGO_ENABLED=0` 交叉编译不受
  影响，部署机不需要 Oracle Instant Client。
- import 了 `storage/oracle` 的业务：编译期需要 CGO（C 编译器），运行期需要 Oracle
  Instant Client（macOS 可用 Config 的 libDir 指向 Instant Client 目录；Linux 配
  ldconfig / LD_LIBRARY_PATH）。
- 单 module 固有现象：业务 `go mod tidy` 后 go.sum 会包含全量依赖校验和（含 godror），
  这是校验文件，不代表链接。若业务对 go.sum 出现 godror 有硬性排斥（如供应链审查），
  可将 `storage/oracle` 拆为嵌套 module 双 replace（当前未做，需要时是纯增量操作）。
- **本库不创建任何后台 goroutine**，所有组件均为同步调用路径。

## 业务系统 CI：层间隔离强制校验

复制以下 depguard 规则到业务仓库 `.golangci.yml`（目录 glob 按业务实际五层调整）：

```yaml
version: "2"
linters:
  settings:
    depguard:
      rules:
        ifaces-no-storage:                # 接口访问层禁存储
          files: ["**/interfaces/**"]
          deny:
            - pkg: "github.com/fab-viewer/common-lib/storage"
              desc: "接口访问层禁止依赖存储包"
        svc-no-infra:                     # 服务/聚合层禁 api+storage
          files: ["**/service/**", "**/domain/**"]
          deny:
            - pkg: "github.com/fab-viewer/common-lib/storage"
              desc: "服务层禁止依赖存储包"
            - pkg: "github.com/fab-viewer/common-lib/api"
              desc: "服务层禁止依赖 API 包"
        storage-no-api:                   # 存储层禁 api
          files: ["**/storage/**"]
          deny:
            - pkg: "github.com/fab-viewer/common-lib/api"
              desc: "存储层禁止依赖 API 包"
        model-slim:                       # 对象层只用跨层包
          files: ["**/model/**"]
          deny:
            - pkg: "github.com/fab-viewer/common-lib/storage"
            - pkg: "github.com/fab-viewer/common-lib/api"
```

注意两点（depguard v2.2 的实际语义，已实测）：
- `deny` 的 pkg 是**前缀匹配**而非 glob——deny `.../common-lib/storage` 即拦截其全部子包；"禁止一切三方依赖"要用 `listMode: strict` + `allow: ["$gostd"]` 白名单表达，写 `github.com/**` 不会命中
- `files` glob 用 `**/目录/**` 形式；结尾再接 `/*.go` 会要求文件位于子目录、顶层文件匹配不到

不用 golangci-lint 的团队可用 Makefile 脚本替代：

```make
check-layers:
	@go list -f '{{.Dir}} {{join .Imports " "}}' ./... | \
	  awk '$$1 ~ /\/interfaces\// && /common-lib\/storage/ {print "违规: " $$0; f=1} \
	       $$1 ~ /\/(service|domain|model)\// && /common-lib\/(api|storage)/ {print "违规: " $$0; f=1} \
	       END {exit f}'
```

## 统一响应体契约（ApiResponse）

成功/失败同构（`response.ApiResponse`，五字段恒序列化），**HTTP 状态码恒
200**（传输层只承载送达，业务结果由 code/success 表达）。**code 是业务
自己的错误码**：OK 默认 **1**、Fail 默认 **0**，经 WithCode 修改；
**txId 取自 RuntimeContext 记录的事务 ID**（请求头 txId 透传优先，未携带
时由 `enrich.Enrich` 生成兜底）；**data 由业务 handler
指定**：single 对象或 multi 对象集合（分页 pageable 整体作为 data）。

```json
{"code":"OK","success":true,"errMsg":"",
 "data":{"pageIndex":2,"pageSize":10,"totalSize":57,"totalPage":6,"data":[...]},
 "txId":"0c8f..."}
```

```json
{"code":"VALIDATION_ERROR","success":false,"errMsg":"参数 email 格式不正确","data":null,"txId":"0c8f..."}
```

- 构建响应只有两个入口：`response.OK(c, data, opts...)`——code 默认
  **"OK"**、success=true、errMsg 默认空串（可 `WithErrMsg` 补充提示）；
  `response.Fail(c, errMsg, opts...)`——code 默认 **"FAIL"**（可
  `WithCode` 换成业务错误码）、success 恒 false、**data 恒 nil**、errMsg
  必填（空串兜底 "internal error"）。常规业务错误走 `c.Error(err)` 交
  fault 统一处理：**BizError 的 code 字符串直接透传响应体 code**（如
  "UserNotFound"），errMsg 直取错误消息（BizError 为 Message，其余为
  err.Error() 原文），出口零翻译
- 分页请求参数为 `pageIndex`/`pageSize`（`request.PageQuery` 解析为
  `paging.PageRequest`；非法回退默认值 **1/10**，pageSize 上限
  `MaxPageSize=100`）；分页出参 **pageable**（`paging.Pageable`：
  pageIndex/pageSize/totalSize/**totalPage**/data）整体作为 `data`，
  totalPage 为 totalSize/pageSize 向上取整（不整除进一、整除不加，
  totalSize=0 为 0）。
- 字符串业务错误码（`bizerror.Code`）与明细（`Details`）不出参：错误码只用于内部映射
  与日志，明细随 `request fault` 失败日志留痕（`fault`）。

### 出参转换（VO → DTO，common/units + response.Wrap）

- time.Time → **yyyyMMddHHmmss** 字符串（`units.FormatDateTime`，与入参
  格式同族；零值转空串）；bool → **int**（`units.BoolToInt`：true→1 /
  false→0）；整数 → 字符串（如雪花 ID 字段）
- `response.Wrap[DTO](vo)`：字段名一致（mapstructure/json tag 优先、忽略
  大小写）自动拷贝 + 上述 wire 转换（嵌套 struct/slice/map/指针递归）；
  其余类型不匹配报错（不做弱转换）。服务层自行组装 DTO 的项目（如
  security-management 的 app 层）只用 units 原语即可

### 请求输入转换约定（common/units + api/request）

| 输入 | wire 约定 | 转换 |
|---|---|---|
| 布尔 | int：**1 → true，其余（含 0）→ false**，int 零值即默认 false | `units.Bool(i)` |
| 日期 | 字符串 `20261021`（yyyyMMdd） | `units.ParseDate(s)` → 当日零点本地时间 |
| 日期时间 | 字符串 `20261021100000`（yyyyMMddHHmmss，与 txTime 头同族） | `units.ParseDateTime(s)` → 本地时间 |
| 毫秒时间戳 | 13 位数字串 `1761012000000` | `units.ParseEpochMillis(s)` → 本地时间 |

- **时间参数解析**：`t, err := request.Time(ctx, "createdAt", c.Query("createdAt"))`（非法
  报公共错误 `InvalidTime`）
  ——8 位日期 / 14 位日期时间 / 13 位毫秒按长度识别；空串按可选参数返回
  零值；非法格式报 VALIDATION（errMsg 带参数名）
- **DTO → service VO**：`request.Wrap[VO](dto)`——字段名忽略大小写对齐自动拷贝
  + 弱类型转换（int → bool 按 units.Bool 约定、时间字符串字段自动解析为
  time.Time）；需要组装/裁剪的转换业务手写
- **必填字符串校验**：`name, err := request.Require(ctx, "username", c.Query("username"))`
  ——去首尾空白，空则公共错误 `ParameterRequired`（commoncodes 字典按请求语言翻译；
  fault 透传 code，HTTP 200）

### 请求运行时上下文（common/rctx.RuntimeContext）

`runtimecontext.Enrich` 解析请求头五个字段
（txId/txTime/txName/userName + 标准头 `Accept-Language`），包装为**整个请求
期间**可用的上下文（`rctx.RuntimeContext`，**字段私有**：WithXxx 链式赋值、
FromXxx 取值），注入请求 context（单路存储，全链路同源）：

| 请求头 | 赋值 / 取值 | 兜底规则 |
|---|---|---|
| `txId` | `WithID` / `FromID` | 缺失生成 32 位 hex，透传优先 |
| `txTime` | `WithTime` / `FromTime` | 格式 yyyyMMDDHHmmss；缺失/解析失败用服务器时间 |
| `txName` | `WithName` / `FromName` | 无（可空） |
| `userName` | `WithUser` / `FromUser` | 无（可空；**即审计操作者**） |
| `Accept-Language` | `WithLang` / `FromLang` | 取 q 值最高的首选语言并规范化（如 `zh-CN`）；缺失/非法为空串，i18n 回退默认语言 |

- **业务层读取**：`rctx.GetRuntimeContext(c.Request.Context())` —— 沿 service/storage
  全链路可读（跨层会合点，与 paging 同模式）
- **审计打通**：`userName` 即审计操作者（RuntimeContext.User），`oracle/repository` 仓储写库时
  `BasePO` 的 `CreatedBy`/`UpdatedBy` 自动填充，业务代码零感知
- **访问日志**：自动携带 `tx_id`/`tx_name`/`user`
- 定时任务无请求头，任务入口自注入：
  `ctx = rctx.SetRuntimeContext(ctx, rctx.RuntimeContext{}.WithID(id).WithName("daily-settle"))`

### 日志（common/logx：Init + 包级函数）

**配置驱动**：application.yml 的 log 段 → `logx.Init(cfgMap)`（唯一初始化
入口，配置结构私有不对外暴露；通常由 cmd/server 统一装配调用）→ 成功后
自动落位全局出口 → 业务经**包级函数**记录，零持有日志对象。引擎为
**zap**，出口两种（console / file，文件滚动为 **lumberjack**），至少启用
一个否则 `ErrNoLogComponent`。

```yaml
# application.yml
log:
  level: info            # 默认等级（四档 debug/info/warn/error）
  console:
    enabled: true
    level: ""            # 空 → 用默认等级
    format: console      # console | json（默认 json；非法取值静默按 json）
  file:
    enabled: true
    format: json
    path: systemlogs/server.log   # 默认值
    maxSize: 100         # 兆，默认 100
    maxBackups: 100      # 默认 100
    maxAge: 30           # 天，默认 30（超期备份清理）
    compress: false      # 默认 false（旧备份 gzip）
    localTime: true      # 默认 true（备份名时间戳时区；false 用 UTC）
```

```go
log, closeLog, err := logx.Init(cfgMap) // 由 cmd/server 装配调用
if err != nil { ... }
defer closeLog()                        // Sync 并关闭文件（优雅停机处）

logx.Info("user created", "id", id)     // 业务代码：包级函数直记
logx.Error("query failed", "err", err)
```

- 等级链：出口 level → log.level → info，**静默兜底不报错**
- **caller 指向业务调用行**（AddCallerSkip(1) + 包级函数直调 zap 实例）
- 测试经 `logx.SetLogger(l)` 注入（Nop 静默或捕获式 fake；注入后包级函数
  自动降级走接口转发）
- Logger 五方法接口仍是库内组件（中间件/存储）的统一接缝；携带固定字段
  用 `logx.Default().With("k", v)`
- 依赖：zap + lumberjack（depguard 已固化白名单）。远程出口（logsink/
  异步批量）与自定义格式机制（LineEncoder）已于 2026-10-21 移除

### 配置加载（common/config）

每业务服务自维护 application.yml，本包只提供加载/读取/校验能力、不内置
任何配置项。`LoadConfig(appName)`：文件路径取环境变量 `{appName}_CFG`
（缺省工作目录 application.yml；仅 yml/yaml/json）→ 解析为嵌套 map →
**叶子环境变量覆盖**（路径点连接加 appName 前缀，如 `security.log.level`；
覆盖值 string 原样进 map，类型转换推迟到绑定）。读取三件套：

```go
m, _ := config.LoadConfig("security")
cfg, err := config.GetSection[oracleconfig.Config](m, "storage.oracle") // 绑定
raw := config.Get(m, "log.file")                                        // 原始子 map，未命中 nil
err = config.ValidateIsNumber(m, "log.file.maxSize")                    // 断言式校验
```

字段匹配：mapstructure tag 优先，无 tag 按完整字段名忽略大小写；
`ValidateIsNumber/IsBoolean` 为宽松判断（"200" 算数字、"true" 算布尔，
与环境变量覆盖机制一致）。

### 统一错误与国际化（common/bizerror + common/i18n + cmd/i18nerr）

`common/bizerror` 提供统一业务错误 **BizError**（Code/Message/Err/Details
四字段，原 FvError/fverr 包 2026-10-21 更名）：
出口只输出 Message，code 直接透传响应体，根因与明细全进日志。**每模块
自己的多语言字典**：文案按语言写成 Go 文件（`messages_<语言>.go`，编译期
进包），生成器产出码常量与 NewError/NewMessage 两个入口——创建错误时按
请求语言查模板插参，**出口不做翻译**，直出 Message。完整示例见
`examples/apifull`（usercodes + ordercodes 双模块）。

```go
// 1. 模块码包内按语言写消息文件 messages_<语言>.go（文件名即语言，key 即码，
//    {0} {1} 为位置占位符，各语言占位符在句中位置可不同）
var MessagesEn = map[string]string{"UserNotFound": "user {0} not found"}
var MessagesZh = map[string]string{"UserNotFound": "用户 {0} 不存在"}

// 2. 生成码常量与翻译入口（与文件 key 一一对齐；包名约定 xxxcodes——
//    包名即模块码命名空间，引用形如 usercodes.UserNotFound）
//    cd usercodes && go run github.com/fab-viewer/common-lib/cmd/i18nerr \
//        -dir . -pkg usercodes -default en

// 3. 错误处一行创建：按请求语言查模板 → 插参 → 补根因明细
return usercodes.NewError(ctx, usercodes.UserNotFound, id). // args 可选，{0} 按调用序替换
    AttachError(rootErr, map[string]any{"id": id})          // 只进日志
// 也可链式补参（与构造 args 等价）：
// usercodes.NewError(ctx, usercodes.UserNotFound).InsertArgs(id)

// 删除确认等带实体名的消息（角色名/用户名/登录策略名同理）：
_ = c.Error(usercodes.NewError(ctx, usercodes.ConfirmDeleteUser).InsertArgs(u.Name))
// → zh："确认删除用户 zhangsan 吗？"  en："confirm deleting user zhangsan?"

// 4. 纯文案出口（不限于错误，可塞进正常业务响应 data）：
tip := usercodes.NewMessage(ctx, usercodes.OrderSubmitted, orderID)
// 不带参取模板，调用点经 i18n.InsertArgs 自行替换：
tip = i18n.InsertArgs(usercodes.NewMessage(ctx, usercodes.OrderSubmitted), orderID)

// 5. 出口（fault）errMsg 直取错误消息；BizError 的 code 直接透传响应体
//    code（HTTP 恒 200，无码表、无映射、无注册）
```

- **唯一底层构造**：`bizerror.NewError(code, message, args...)`——Message 直传、
  args 按序替换 {0} {1} 占位符（模块 NewError 内部先查字典再交给它；基础设施层
  直传 message，根因经 `.AttachError(err, nil)` 链上）
- **判断**：`bizerror.IsError(err)`、`bizerror.From(err)`（errors.Is/As 全兼容）
- **语言来源**：标准头 `Accept-Language`（q 值降序取首选并规范化），经
  `enrich.Enrich` 注入 `rctx.RuntimeContext.Lang`，全链路可读
- **回退链**：请求语言精确 → 基础语言（`zh-CN` → `zh`，只收窄不放宽）→
  模块默认语言 → key 原文
- 多模块各持字典互不感知（无全局注册表，同 key 跨模块互不冲突）；表并发安全
- **code 是字符串、与 bizerror.Code 同一命名空间**（统一业务码体系）：
  BizError 的 code 由 fault 直接透传进响应体，非 BizError 为默认 "FAIL"；
  OK 默认 "OK"。HTTP 状态码恒 200。fault 出口不做翻译或固定文案替换
  （BizError errMsg 取 Message，其余取 err.Error() 原文——未知错误根因直出
  响应体，业务方拍板取舍；失败日志带调用栈留痕）
- 未知错误的 errMsg 为 err.Error() 原文直出响应体（根因随 `request fault`
  日志带调用栈留痕）
- `gorm.ErrRecordNotFound` / `sql.ErrNoRows` 不做内置翻译（errMsg 直出原文）。

## 各存储包部署前提与用法

详见各包 godoc（`storage/oracle`、`storage/opensearch` 的
doc.go 各有完整用法示例）。要点：

| 包 | 部署前提 | 关键 API |
|---|---|---|
| `storage/oracle`（conn/config/base/repository 四子包） | CGO + Oracle Instant Client；GORM/原生 SQL/存储过程三通道 | `conn.Open` / `conn.Init`（组件自治装配）/ `db.GORM()` / `db.SQL()` / `db.CallProc` / `conn.CallProcTx` |
| `storage/opensearch`（config/search 子包） | 无（HTTP，默认 9200；自签集群可开 Insecure） | `Open` / `Init`（组件自治装配）/ `search.Search[T]` / `client.Index` / `client.Get[T]` / `client.Delete` / `client.SDK()` |

**组件自治装配（Init）**：oracle/opensearch 与 logx 同款约定——`Init(配置map)`
读各自配置段（`oracle` / `opensearch`），**段缺失即未启用**（返回 nil 句柄，
回退由业务自决；环境变量覆盖只能改写文件已有叶子，不能凭空建段）；段存在
则绑定校验建连。不进 server.ProviderSet（存储是可选件），业务组装处一行
调用，cleanup 接入 wire cleanup 链（oracle 承载 Close，OS 为空操作）：

```yaml
# application.yml（camelCase 键直命中，时长带单位）
oracle:
  user: security
  password: secret          # 敏感值经 security.oracle.password 环境变量覆盖注入
  host: db.internal
  serviceName: FREEPDB1
  pool: {connMaxLifetime: 1h}
opensearch:
  addrs: [https://os1:9200]
  insecure: false
```

### 事务使用约定（Oracle）

- 常规事务：`db.GORM().WithContext(ctx).Transaction(func(tx *gorm.DB) error { ... })`
- 存储过程参与同一事务：在 Transaction 回调内用 `conn.FromGorm(tx)` 取 Execer，
  再 `conn.CallProcTx(ctx, execer, "P_X", ...)`——过程与 GORM DML 共享同一 `*sql.Tx`。
- 嵌套事务用 GORM 的 SavePoint；不要在回调外保存 tx 使用。
- 提醒：`GORM()` 出口返回裸 `*gorm.DB`，链式操作记得 `.WithContext(ctx)`。

### 数据库迁移约定（goose）

- 库本身不做迁移；业务系统统一用 [goose](https://github.com/pressly/goose) 管理版本化
  DDL，迁移文件随业务仓库走。
- 存储过程/函数的 DDL 必须用 `-- +goose StatementBegin/End` 包裹（PL/SQL 块含分号）。
- 本库集成测试自带 goose 迁移目录（`storage/oracle/conn/testdata/migrations/oracle/`），
  可作为业务系统的样板。

## 集成测试（本库开发用）

```bash
docker compose -f docker-compose.test.yml up -d
export COMMON_LIB_ORACLE_DSN='commonlib/commonlib@localhost:1521/freepdb1'
export COMMON_LIB_OS_ADDRS='http://localhost:9200'
make integration          # go test -tags=integration ./...
```

环境变量缺失的存储自动 skip；Oracle 容器初始化约需 1-2 分钟。

## 版本策略与 module 名迁移

- 业务系统按 **git tag** 锁版本（v0.1.0 起），升级前看 [CHANGELOG](CHANGELOG.md)。
- 库的依赖升级必须全量测试通过后单独提交并记录 CHANGELOG；`godror` 处于 v0.x
  （语义化版本允许 breaking），升级必须跑 `make integration` 的 Oracle 全量线。
- 当前 module 名 `github.com/fab-viewer/common-lib` 为**占位名**：业务方 replace
  本地引用起步。未来落到真实托管路径时：改本仓 module 名 + 业务仓批量替换 import
  （`gofmt -r` 一次跑完）+ 删 replace 行，公共 API 零变化。

## 开发（本仓库）

```bash
make lint      # golangci-lint v2（含库内 depguard 架构防腐）
make test      # go test -race（CI 必含 race 检测）
make cover     # 覆盖率门槛：整体 99%
make mock      # go generate（mockgen）
make integration
make fmt       # gofmt + goimports（提交前跑）
```

覆盖率分级：跨层包（common/bizerror、common/paging、common/logx、common/config）与 api ≥99%；存储包装层 ≥95%（握手类路径由
集成测试兜底）。

## 示例

| 目录 | 演示 |
|---|---|
| `examples/apifull` | API 全家桶（四中间件+统一响应+分页+优雅停机），可直接 go run |
| `examples/oracleproc` | Oracle：Open→GORM CRUD→CallProc 标量 OUT→游标→Close |
| `examples/ossearch` | OpenSearch：Index→Search（分页+排序）→Get→Delete 生命周期 |

示例兼作公共 API 烟雾测试（进 CI 编译）；存储类示例缺环境变量时打印用法并以 0 退出。
