# common-lib

fab-viewer 体系的 Go 公共库：集中提供**存储访问**（Oracle / ClickHouse / OpenSearch）
与 **API 公共组件**（统一响应、分页、错误处理、日志中间件），被两类业务系统 import：

- **RESTful API 业务**（Gin）
- **内部定时器业务**（无接口访问层）

## 版本与环境要求

| 项 | 要求 |
|---|---|
| Go 工具链 | **≥ 1.26**（受 opensearch-go/v4 最低版本约束） |
| Oracle 访问 | 仅当 import `storage/oracle` 时：编译期 CGO + C 编译器；运行期 Oracle Instant Client |
| 其他存储 | 无额外系统依赖（clickhouse-go 原生协议 / opensearch HTTP） |

## 包 ↔ 业务分层映射矩阵（核心）

业务系统分层：接口访问层 / 服务层 / 业务聚合层 / 存储层 / 对象层（定时器业务无接口访问层）。
**import 路径即能力边界**：

| common-lib 包 | 接口访问层 | 服务层 | 业务聚合层 | 存储层 | 对象层 | main 组装根 |
|---|:---:|:---:|:---:|:---:|:---:|:---:|
| `errs` / `paging` / `logx`（跨层，零三方依赖） | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| `api`、`api/middleware` | ✓ | ✗ | ✗ | ✗ | ✗ | ✓ |
| `storage/oracle` / `storage/clickhouse` / `storage/opensearch` | ✗ | ✗ | ✗ | ✓ | ✗ | ✓ |

约定：

- **组装根豁免**：业务 `main` 不属于五层，可同时 import 两边（如 `/healthz` 由 main
  把存储层的 `Ping` 实现注入路由）。
- **SDK 类型不越层**：`*gorm.DB`、`*opensearch.Client` 等只允许出现在业务存储层内部，
  存储层对上的接口签名用业务自定义类型 + `errs`/`paging` 类型。
- **跨层共享靠依赖倒置**：接口层 `api.PageQuery(c)` 产出 `paging.Query` 沿层传递，
  存储层消费 `q.Limit()/q.Offset()` 或 `opensearch.WithPaging(q)`；`api` 与 `storage`
  互不感知，只在 `paging`/`errs` 相会。

## 快速开始

业务系统 `go.mod`（正式托管前的本地引用方式）：

```go
module your-business-system

require github.com/fab-viewer/common-lib v0.0.0

replace github.com/fab-viewer/common-lib => ../../services/common-lib
```

### RESTful API 业务（接口访问层 + 存储层）

```go
r := gin.New()
// 注册顺序固定：AccessLog → ErrorHandler → Recovery → RequestID
//（AccessLog 必须在 ErrorHandler 外层，否则错误路径记录的 status 不正确）
r.Use(
    middleware.AccessLog(log),
    middleware.ErrorHandler(log),
    middleware.Recovery(log),
    middleware.RequestID(),
)

r.GET("/api/users", func(c *gin.Context) {
    q := api.PageQuery(c) // 解析 ?page=&size=，非法值回退默认
    res, err := userRepo.List(c.Request.Context(), q) // 返回 paging.Result[User]
    if err != nil {
        _ = c.Error(err) // 交给 ErrorHandler 统一写回，不要自行写错误响应
        return
    }
    api.OKPage(c, res)
})
```

### 定时器业务（无接口访问层，只 import 存储包 + 跨层包）

```go
db, err := oracle.Open(ctx, oracle.Config{
    User: "app", Password: "***", Host: "db.example.com", ServiceName: "ORCLPDB1",
})
if err != nil { ... }

var n int64
err = db.CallProc(ctx, "P_ACCRUE", jobID, oracle.Out(&n))
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
          files: ["**/interfaces/**/*.go"]
          deny:
            - pkg: "github.com/fab-viewer/common-lib/storage"
              desc: "接口访问层禁止依赖存储包"
        svc-no-infra:                     # 服务/聚合层禁 api+storage
          files: ["**/service/**/*.go", "**/domain/**/*.go"]
          deny:
            - pkg: "github.com/fab-viewer/common-lib/storage"
              desc: "服务层禁止依赖存储包"
            - pkg: "github.com/fab-viewer/common-lib/api"
              desc: "服务层禁止依赖 API 包"
        storage-no-api:                   # 存储层禁 api
          files: ["**/storage/**/*.go"]
          deny:
            - pkg: "github.com/fab-viewer/common-lib/api"
              desc: "存储层禁止依赖 API 包"
        model-slim:                       # 对象层只用跨层包
          files: ["**/model/**/*.go"]
          deny:
            - pkg: "github.com/fab-viewer/common-lib/storage"
            - pkg: "github.com/fab-viewer/common-lib/api"
```

不用 golangci-lint 的团队可用 Makefile 脚本替代：

```make
check-layers:
	@go list -f '{{.Dir}} {{join .Imports " "}}' ./... | \
	  awk '$$1 ~ /\/interfaces\// && /common-lib\/storage/ {print "违规: " $$0; f=1} \
	       $$1 ~ /\/(service|domain|model)\// && /common-lib\/(api|storage)/ {print "违规: " $$0; f=1} \
	       END {exit f}'
```

## 统一响应体契约

成功/失败同构（`api.Body`）：

```json
{"code":"OK","message":"success","data":{"total":57,"page":2,"size":5,"items":[...]},
 "requestId":"0c8f...","timestamp":"2026-10-14T17:53:31.98+08:00"}
```

```json
{"code":"VALIDATION_ERROR","message":"参数 email 格式不正确","details":{"field":"email"},
 "requestId":"0c8f...","timestamp":"2026-10-14T17:53:31.98+08:00"}
```

默认错误码 → HTTP 状态码（`api.DefaultStatusMap`，可经 `middleware.WithStatusMap` 增补）：

| code | HTTP |
|---|---|
| OK | 200 |
| VALIDATION_ERROR | 400 |
| UNAUTHORIZED | 401 |
| FORBIDDEN | 403 |
| NOT_FOUND | 404 |
| CONFLICT | 409 |
| RATE_LIMITED | 429 |
| INTERNAL_ERROR / DB_ERROR | 500 |
| UPSTREAM_ERROR | 502 |

- 业务扩展码自带前缀（如 `const CodeUserDisabled errs.Code = "USER_DISABLED"`），
  经 `WithStatusMap` 映射状态码；未映射的自定义码按 500 处理。
- 未知错误的根因只进日志不进响应（`ErrorHandler` 统一兜底）。
- `gorm.ErrRecordNotFound` / `sql.ErrNoRows` 在 `ErrorHandler` 中内置翻译为 404。

## 各存储包部署前提与用法

详见各包 godoc（`storage/oracle`、`storage/clickhouse`、`storage/opensearch` 的
doc.go 各有完整用法示例）。要点：

| 包 | 部署前提 | 关键 API |
|---|---|---|
| `storage/oracle` | CGO + Oracle Instant Client；GORM/原生 SQL/存储过程三通道 | `Open` / `GORM()` / `SQL()` / `CallProc` / `CallProcTx` |
| `storage/clickhouse` | 无（原生 TCP 协议，默认 9000 端口） | `Open` / `NewBatch[T]` / `QuerySlice[T]` / `Conn()` |
| `storage/opensearch` | 无（HTTP，默认 9200；自签集群可开 Insecure） | `Open` / `Search[T]` / `Index` / `Get[T]` / `Delete` / `SDK()` |

### 事务使用约定（Oracle）

- 常规事务：`db.GORM().WithContext(ctx).Transaction(func(tx *gorm.DB) error { ... })`
- 存储过程参与同一事务：在 Transaction 回调内用 `oracle.FromGorm(tx)` 取 Execer，
  再 `CallProcTx(ctx, execer, "P_X", ...)`——过程与 GORM DML 共享同一 `*sql.Tx`。
- 嵌套事务用 GORM 的 SavePoint；不要在回调外保存 tx 使用。
- 提醒：`GORM()` 出口返回裸 `*gorm.DB`，链式操作记得 `.WithContext(ctx)`。

### 数据库迁移约定（goose）

- 库本身不做迁移；业务系统统一用 [goose](https://github.com/pressly/goose) 管理版本化
  DDL，迁移文件随业务仓库走。
- 存储过程/函数的 DDL 必须用 `-- +goose StatementBegin/End` 包裹（PL/SQL 块含分号）。
- 本库集成测试自带 goose 迁移目录（`storage/oracle/testdata/migrations/oracle/`），
  可作为业务系统的样板。

## 集成测试（本库开发用）

```bash
docker compose -f docker-compose.test.yml up -d
export COMMON_LIB_ORACLE_DSN='commonlib/commonlib@localhost:1521/freepdb1'
export COMMON_LIB_CH_DSN='clickhouse://commonlib:commonlib@localhost:9000/commonlib'
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

覆盖率分级：跨层包（errs/paging/logx）与 api ≥99%；存储包装层 ≥95%（握手类路径由
集成测试兜底）。

## 示例

| 目录 | 演示 |
|---|---|
| `examples/apifull` | API 全家桶（四中间件+统一响应+分页+优雅停机），可直接 go run |
| `examples/oracleproc` | Oracle：Open→GORM CRUD→CallProc 标量 OUT→游标→Close |
| `examples/chbatch` | ClickHouse：Batch 流式写入→QuerySlice 读回比对 |
| `examples/ossearch` | OpenSearch：Index→Search（分页+排序）→Get→Delete 生命周期 |

示例兼作公共 API 烟雾测试（进 CI 编译）；存储类示例缺环境变量时打印用法并以 0 退出。
