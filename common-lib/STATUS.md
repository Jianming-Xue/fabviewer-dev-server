# 当前状态（STATUS）

> 快照日期：2026-10-21。本文件记录仓库的实时状态与待办，历史沿革见
> [CHANGELOG](CHANGELOG.md)，完整规范见 [README](README.md)，交接说明见
> [HANDOVER](HANDOVER.md)。

## 一句话状态

common-lib 五项核心能力改造、API 层分包、请求参数转换、cmd/server 统一
装配均已完成并推送；响应契约已定型 ApiResponse（HTTP 恒 200、code 为字符串业务码与
bizerror.Code 同一命名空间：OK "OK"/Fail "FAIL"/BizError 透传，OK/Fail
唯一两入口）；请求上下文已收敛单路存储（reqctx 包删
除，rctx 直取）；四中间件已按业务方拍板定名 observer/fault/guard/enrich；存储组件 Init 形态待设计；snowflake 仍是 WIP。

## 已完成（2026-10-21 会话，已提交推送）

| 轮次 | 内容 | 验证 |
|---|---|---|
| 6. 配置加载 `common/config` | LoadConfig（{appName}_CFG→application.yml；yml/yaml/json；叶子环境变量覆盖 security.log.level 式，值 string 原样进 map）、GetSection[T]（mapstructure/v2 绑定，WeaklyTypedInput）、Get、ValidateIsEmpty/IsNumber/IsBoolean（宽松判断）；新增依赖 mapstructure/v2，yaml.v3 转 direct | 覆盖率 100% + lint 0 |
| 7. logx 重构 | Init(m) 唯一入口（配置结构私有、等级链 outlet→global→info 静默兜底、format json 默认/console）、console/file 双出口、滚动换 lumberjack（maxAge/compress/localTime）、包级 Debug/Info/Warn/Error（独立 log.go，caller 指向业务行）、SetLogger（原 SetDefault）、ErrNoLogComponent；**删除**：远程出口（Sink/异步缓冲/logsink 包）、自定义格式（LineEncoder/Formatter/Rec）、自研编号滚动（推翻 2026-10-20 拍板）、NewSlog（slog 适配） | 覆盖率 100% + lint 0 + security-management 真机实测（双格式日志、caller 指向 main.go 业务行） |
| 8. 统一错误更名 | fverr → bizerror（FvError → BizError，仅更名）；i18nerr 生成器随迁，examples 双模块 + security-management 的 errors_gen 重生成零漂移 | 全库 build/vet/test |
| 9. actortx 并入 rctx | actortx 包删除（操作者即 RuntimeContext.User，双通道合一）；RuntimeContext 字段私有化（WithXxx 链式赋值值语义 / FromXxx 取值）；审计钩子/accesslog/语法糖全量随迁 | 全库 race 测试 |
| 16. 运行时上下文收敛单路 + 删 reqctx 包 | RuntimeContext 只存请求 context 一份（FillRuntimeContext 替换 c.Request；读取方一律 rctx.GetRuntimeContext(c.Request.Context()) 直取）；**api/reqctx 包删除**（头常量移入 runtimecontext 中间件包=生产方；RuntimeContextOf/TxIDOf/LangOf 语法糖删——rctx 直取；WithActor 删——认证中间件按文档样例手写一行 rctx.SetRuntimeContext）；消除 WithActor 更新后 gin 侧第二份存储不漂移的隐患，新增守护测试（认证更新操作者→全链路见新值）| api 覆盖率 100% + lint 0 + race 全绿 + SM 构建通过 |
| 19. code 统一字符串 | ApiResponse.Code int→**string**（与 bizerror.Code 同一命名空间）：OK 默认 **"OK"**、Fail 默认 **"FAIL"**；fault 处理 BizError 时 **code 直接透传**（WithCodeMap 机制删除）、非 BizError 默认 "FAIL"；server.NewEngine 删 codeMap 参数、SM usererrors.CodeMap() 删除、模块码零注册零映射 | api 覆盖率 100% + lint 0 + race 全绿 + SM 3 连过 + 冒烟实测（"code":"OK" / "code":"UserNotFound" 透传）|
| 18. errorhandler→fault 职责简化 | 统一错误出口更名 fault：**不管 BizError 与否一律记失败日志+调用栈**（bizCode/details 进日志留痕），经 response.Fail 写回——**errMsg 直取错误消息零翻译**（BizError 取 Message，其余 err.Error() 原文直出响应体）；删哨兵翻译（gorm→404/canceled→499/其他→internal error 固定文案）与 WithMessageFunc 出口改写；WithCodeMap 保留 |
| 17. 四中间件更名（业务方拍板） | accesslog→**observer**（Observe，访问观察）、recovery→**guard**（Guard，守护兜底）、runtimecontext→**enrich**（Enrich，上下文丰富；用户输入 Erich 按 Enrich 语义落地）——包名/函数/文档全量随迁，目录 git mv 保留历史 | api 覆盖率 100% + lint 0 + race 全绿 + SM 构建通过 |
| 16. 运行时上下文收敛单路 + 删 reqctx 包 | request.Time(name, v)（8 位日期/14 位日期时间/13 位毫秒按长度识别；空串可选参数零值；非法报 VALIDATION 带参数名与原值）+ Wrap 时间钩子（DTO 时间字符串字段 → VO time.Time 自动解析）| api 覆盖率 100% + lint 0 + race 全绿 |
| 14. 响应契约重塑 ApiResponse | Response→ApiResponse、Contents→Data；**HTTP 恒 200，code 为业务码（OK 默认 1 / Fail 默认 0，WithCode 可改）**；构建只有 OK/Fail 两入口（write 私有，errorhandler/recovery 库内也走 Fail）；删 Created/NoContent/OKPage/NewResponse；WithStatusMap→**WithCodeMap**（模块码→业务 code，未映射 0，库内无默认业务码表，**errresp 包删除**）；新增 response.Wrap[DTO]（VO→DTO：time→yyyyMMddHHmmss（与入参同格式）、bool→int、整数→字符串，嵌套递归）+ units.FormatDateTime/BoolToInt；SM 随迁（Enabled 出入参 int、handler 统一 OK、StatusMap→CodeMap 值语义变业务码）| api 覆盖率 100%（门槛 99%）+ lint 0 + race 全绿 + SM 端到端实测（创建 code:1/enabled:1/时间紧凑格式；404 → HTTP 200 + body code:404）|
| 13. cmd/server 统一装配 | 中间件零持有日志（三中间件去 log 参数，经 logx 全局出口；testutil.CaptureLog）；新增 cmd/server 包：NewEngine（四中间件固定顺序+/healthz，模块码表唯一必传差异）、Init（server 段：addr/mode/四超时/shutdownTimeout，段缺失全默认，gin.SetMode 副作用）、NewServer+Run（cleanup 优雅停机）、ProviderSet（wire 方案 A，engine 归业务）；common/config 增 Bind（保留已有值/出现的键覆盖）+ 时长钩子（GetSection 同步挂）；security-management 迁 wire（wire.go/providers.go/瘦身 main，SM_ADDR→server 段+security.server.addr 覆盖）；apifull 改手工全家桶示例 | 双模块 build/vet/test(race) 全绿 + lint 0（除 snowflake WIP）+ SM 端到端冒烟（healthz/创建/列表/环境变量覆盖 :8099/优雅停机链）+ apifull 冒烟（中文 404/停机日志）|
| 12. 请求参数转换 | 新增 common/units（Bool：int 1→true 其余 false；ParseDate/ParseDateTime：8/14 位日期字符串→本地时间；ParseEpochMillis：13 位毫秒）；api/request 增 Wrap[VO]（DTO→VO 字段名对齐+弱类型，int→bool 按 units.Bool 钩子）与 Require（必填去空白→VALIDATION）；runtimecontext txTime 解析改用 units（时区 UTC→本地）；depguard 白名单/测试豁免随迁 | units 100%、api 聚合 99.1%、lint 0（除 snowflake WIP）、race 全绿 |
| 11. API 层按功能分包 | `api` 根包拆为 reqctx（gin↔rctx 绑定+WithActor）/ request（PageQuery）/ response（apiresponse 写回）/ errresp（Fail+码表）；`api/middleware` 按处理流程点拆 observer/fault/guard/enrich 四子包（根包留注册顺序契约+全栈测试）；新增 api/internal/testutil 共享测试脚手架；accesslog.AccessLogOption→Option；examples/apifull 与 security-management 随迁 | build/vet/test(race) 全绿 + lint 除 snowflake 3 条 WIP 外零告警 + apifull 端到端冒烟（分页/404/txId）|
| 10. 分页契约 | Query→PageRequest、默认 pageSize 20→**10**、MaxSize 500→**MaxPageSize=100**、Pageable 增加 **totalPage**（totalSize/pageSize 向上取整，整除不加）、出参字段顺序 pageIndex/pageSize/totalSize/totalPage/data | 契约测试更新 + 真机 wire 实测 |
| 业务随迁 | security-management：新增 application.yml，cmd/server 经 config.LoadConfig("security")+logx.Init 装配；oracleproc/apifull 示例随迁 | memory 模式端到端实测（创建/分页/双格式日志/caller） |

当前包结构：

```
common-lib/
├── common/          bizerror / paging / rctx（跨层零依赖会合点）
│                    config（配置加载：yml/yaml/json + 环境变量覆盖 + mapstructure 绑定）
│                    logx（日志完整工具：Init + 包级函数，zap + lumberjack 双出口）
│                    i18n（多语言表能力，仅依赖 common/rctx）
├── cmd/i18nerr/     错误变量集合生成器（messages_<语言>.go → errors_gen.go）
├── api/             按功能分包：request（请求参数解析/校验/时间/包装）/
│                    response（ApiResponse 统一响应 + VO→DTO Wrap）/
│                    internal/testutil（共享测试脚手架）
├── api/middleware/  按处理流程点分包：observer / fault / guard /
│                    enrich / fault（根包仅注册顺序契约+全栈测试）
├── storage/
│   ├── pool.go      PoolConfig
│   ├── snowflake/   点分隔雪花 ID（WIP，见已知问题）
│   ├── oracle/      根=向导；conn / config / base / repository
│   ├── clickhouse/  根=Client/Open/QuerySlice；config / batch
│   └── opensearch/  根=Client/Open/Index/Get/Delete；config / search（logsink 已删）
└── examples/        apifull / oracleproc / chbatch / ossearch（兼作编译烟雾测试）
```

## Git 状态

- 上述 6–10 轮已提交推送（c894212 + 清理提交 66215f8）；最后 tag 仍为
  `v0.5.1`（Unreleased 已积累**大量** breaking，接入业务前建议打 v0.6.0）
- 业务服务 security-management 的 application.yml 进仓库根（配置样板），
  叶子可经 `security.*` 环境变量覆盖

## 待定决策

- **txId 缺失行为**：header 未带 txId 时当前为「crypto/rand 生成 32 位 hex
  兜底」。规则待业务方拍板，改动点集中在 `api/middleware/enrich/enrich.go`
  一处（注释已标注）
- **存储组件 Init 形态**（cmd/server 拍板项仅剩此条）：各存储组件从
  config map 初始化的配置段与 enabled 判断方式（组件自治已定；四中间件
  注册方式与 server 段已于 2026-10-21 拍板落地，详见 HANDOVER 第 6 节）

## 已知问题

| 问题 | 位置 | 影响 |
|---|---|---|
| 测试数据竞争：`nowFn` 钩子无同步写入 | `storage/snowflake/snowflake_test.go:134`（写）vs `snowflake.go:104`（读） | `make test`（-race）失败；需改 atomic |
| **同毫秒序列缺陷：同毫秒内两次 Next() 序列均为 0 → ID 重复** | `storage/snowflake/snowflake.go` | 存量逻辑缺陷；security-management 的 `app/user_test.go` **间歇失败实证**（同毫秒两用户撞 ID 后者覆盖前者，totalSize 断言失败；跨毫秒则通过） |
| 单测覆盖率 86.8% | `storage/snowflake` | `make cover`（99% 门槛）不过 |
| 3 条 lint 告警 | snowflake×3（epochMs 未用、switch 可 tagged、real 遮蔽内建名） | `make lint` 非零退出 |
| `repository` 包无任何测试 | `storage/oracle/repository` | Create/Update 落库路径无保护 |

## 质量门槛现状

- `make lint`：除 snowflake 3 条 WIP 外零告警（本轮 API 分包 0；另补上
  rctx FromUser/FromLang 漏标的 hugeParam nolint×2，恢复既定状态）
- `make test`：除 snowflake race 外全部通过（common/api/存储全绿，
  config/logx 覆盖率 100%）
- `make cover`：受 snowflake 拖累暂不过；其余包达标（跨层包与 api ≥99%）
- `make integration`：本轮五项改造后**尚未跑**集成线（建议提交 tag 前补跑）
