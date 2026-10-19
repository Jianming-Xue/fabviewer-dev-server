# 交接文档（HANDOVER）

> 面向接手本库的工程师：项目是什么、怎么组织的、三大契约长什么样、
> 怎么开发验证、还欠什么。规范细节以 [README](README.md) 为准，
> 实时状态与待办见 [STATUS](STATUS.md)，变更史见 [CHANGELOG](CHANGELOG.md)。

## 1. 项目是什么

fab-viewer 体系的 Go 公共库（单 module：`github.com/fab-viewer/common-lib`，
module 名为占位、业务方经 `replace` 本地引用）。被两类业务系统 import：

- **RESTful API 业务**（Gin）：用 `api` + `api/middleware` + 存储包
- **内部定时器业务**（无接口层）：只用跨层包 + 存储包

硬约束：**零后台 goroutine**、全同步调用路径、import 路径即能力边界。

## 2. 核心设计理念（务必先懂再动）

1. **import 路径即能力边界**：`api`/`api/middleware` 只准接口访问层 import
   （Gin 依赖从这里开始）；`storage/*` 只准存储层 import；`common/*` 跨层通用。
   由 `.golangci.yml` depguard 强制（deny 是前缀匹配，能命中全部子包）。
2. **跨层会合点**：`api` 与 `storage` 互不感知，只在小包相会——
   `errs`（错误语义）、`paging`（分页对象）、`actor`（操作者）、`tx`（事务上下文）。
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
common/errs      Code 错误码、Error（Wrap/New/From）、sentinel
common/paging    Query{PageIndex,PageSize}、Pageable[T]（出参 pageable）
common/logx      Logger 五方法接口、slog 适配、全局默认出口
common/actor     操作者 context 传递（审计字段的来源）
common/tx        Tx{ID,Time,Name,User} 运行时上下文（请求头四字段）
api              Response/OK/OKPage/Fail、PageQuery、DefaultStatusMap
api/middleware   AccessLog → ErrorHandler → Recovery → RuntimeContext（顺序固定）
storage/oracle/conn        Open、DB 三通道（GORM/SQL/存储过程）、Cursor、driverAdapter
storage/oracle/config      Config/WithDefaults/Validate/BuildDSN
storage/oracle/base        BasePO 审计基类（GORM 钩子自动填 CreatedBy 等）
storage/oracle/repository  Repository[T]（雪花 ID + 审计落库）
storage/clickhouse(+config/batch)   Open/Client/QuerySlice；Batch[T] 批量写
storage/opensearch(+config/search)  Open/Client/Index/Get/Delete；Search[T] 泛型搜索
storage/snowflake  点分隔雪花 ID（"<Unix毫秒>.<节点>.<序列>"）
storage/pool.go    PoolConfig 连接池配置
```

依赖方向（无环）：config ← conn ← repository；base 只依赖 actor+gorm；
batch/search 依赖各自根包；所有存储子包互不依赖（depguard 强制）。

## 4. 三大对外契约（wire 格式）

### 4.1 统一响应 apiresponse（五字段恒序列化，成功/失败同构）

```json
{"code":200,"success":true,"errMsg":"","contents":{...},"txId":"abc"}
{"code":404,"success":false,"errMsg":"user 999 not found","contents":null,"txId":"abc"}
```

- `code` = HTTP 状态码（数字），由 `errs.Code → DefaultStatusMap` 映射产生
- 字符串错误码与 Details **不出参**，随 `business error` 日志留痕
- 业务错误统一走 `c.Error(err)` → ErrorHandler，handler 不自写错误响应

### 4.2 分页

- 入参：`?pageIndex=2&pageSize=20`（`api.PageQuery` 解析；非法回退 1/20，
  pageSize 上限 500）
- 出参（置于 contents）：`{"pageSize":20,"pageIndex":2,"totalSize":57,"data":[...]}`

### 4.3 请求运行时上下文

请求头四字段 → `middleware.RuntimeContext` → `tx.Tx` 三路注入：

| header | tx.Tx | 兜底 |
|---|---|---|
| txId | ID | 缺失生成 32 hex（**行为待定**，改 `runtimecontext.go` 一处） |
| txTime | Time | yyyyMMDDHHmmss；缺失/非法→服务器时间 |
| txName | Name | 可空 |
| userName | User | 可空；同时注入 actor → oracle BasePO 审计自动填充 |

业务层读取：`tx.FromContext(c.Request.Context())`。

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

1. **提交当前变更**：四轮变更全在工作区未提交，建议按 STATUS.md 的四轮拆分
   commit，CHANGELOG Unreleased 段已分条对应
2. **snowflake WIP 修复**：测试 race（`nowFn` 改 atomic）+ 3 条 lint + 覆盖率
   补齐——修完 `make test/cover/lint` 三门槛即全部恢复
3. **repository 测试**：baseOf 校验可单测（参照 `base/audit_test.go` 的封印
   接口写法）；Create/Update 落库用 sqlmock（conn 包 `testutil_test.go` 有
   现成 connDialector 可参考）或进集成线
4. **txId 缺失行为拍板**：业务方定规则后改 `runtimecontext.go` 一处
5. **集成线补跑**：结构调整后单测全绿，Oracle/CH/OS 真环境链路建议提交前跑
   `make integration`（v0.5.1 时曾靠它抓出四处真库问题）
6. （远期）module 名迁移到真实托管路径：改 module 名 + 业务仓 `gofmt -r`/
   sed 批量替换 import + 删 replace，公共 API 零变化

## 7. 约定速记（评审必查）

- handler 出错：`c.Error(err)` + return，不写响应；未知错误根因只进日志
- GORM 出口裸 `*gorm.DB`，链式调用记得 `.WithContext(ctx)`
- 事务内调存储过程：`conn.FromGorm(tx)` 取 Execer → `conn.CallProcTx`
- 新增跨层共享概念：在 `common/` 加零依赖小包并加进 depguard strict 白名单
- 依赖升级必须全量测试后单独提交并记 CHANGELOG；godror 处于 v0.x，
  升级必跑 Oracle 集成线
- 响应体/分页/上下文的任何字段调整都是 Breaking Change，记 CHANGELOG 并同步
  README 契约章节与 STATUS
