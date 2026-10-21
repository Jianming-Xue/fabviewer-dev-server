# 仓库交接文档（services / fabviewer-dev-server）

> 面向接手人：仓库长什么样、两个模块什么关系、当前进度到哪、下一步做什么。
> 各模块细节见各自的 HANDOVER/README/STATUS/CHANGELOG。

## 1. 仓库拓扑

单 git 仓库（远端 `github.com:Jianming-Xue/fabviewer-dev-server.git`，分支
main），**两个独立 Go module 并列**（2026-10-20 由原 common-lib 单仓库
根上移重组而来，历史经 rename 完整保留）：

```
services/
├── common-lib/              公共库（module github.com/fab-viewer/common-lib）
│   └── 详见 common-lib/HANDOVER.md（设计理念/包地图/契约/验证流程）
└── security-management/     安全管理服务（module github.com/fab-viewer/security-management）
    ├── application.yml      服务配置样板（appName=security，叶子可经 security.* 环境变量覆盖）
    └── 详见 security-management/README.md（分层结构/快速开始/差异点记录）
```

服务经 `replace github.com/fab-viewer/common-lib => ../common-lib` 引用
公共库（相对路径，挪根后仍成立）。新增业务服务照此并列。

## 2. 跨模块约定（2026-10-21 会话刷新，改动前必读）

| 约定 | 内容 |
|---|---|
| 服务配置 | 每服务自维护 `application.yml`（路径可经 `{appName}_CFG` 覆盖）；加载/读取/校验用 `common/config`（LoadConfig/GetSection[T]/Bind/Get/ValidateIs*）；叶子环境变量覆盖：`security.log.level` 式点分全路径 |
| HTTP 装配 | 引擎/四中间件/healthz/优雅停机归 `common-lib/cmd/server`（NewEngine 传模块码表 + Init 读 server 段 + NewServer/Run）；业务经 wire `server.ProviderSet` 装配，main 只剩 InitializeServer → Run；server 段默认 :8080/release，超时写 "5s" 式字符串 |
| 日志 | `logx.Init(cfgMap)` 唯一初始化（cmd/server 统一装配调用）；业务一律包级函数 `logx.Info/Warn/Error/Debug` 零持有；双出口 console/file（lumberjack 滚动）、等级链 outlet→global→info 静默兜底、format json 默认/console；caller 指向业务行；测试经 SetLogger 注入 |
| 统一响应 | `response.ApiResponse` 五字段（code/success/errMsg/data/txId），**HTTP 恒 200**；构建仅 `OK`/`Fail`：code 是业务码（OK 默认 **1**、Fail 默认 **0**，WithCode 可改），OK 的 errMsg 可选、Fail 的 errMsg 必填且 data 恒 nil；data 由 handler 给（single/集合/分页 pageable）；模块错误码经 `errorhandler.WithCodeMap` 映射业务 code；VO→DTO 出参 time→yyyyMMddHHmmss 字符串、bool→int（response.Wrap 或 units 原语） |
| 统一错误 | `bizerror.BizError` 四字段（原 fverr/FvError 更名）；唯一构造 `NewError(code,msg,args...)` + 链式 `InsertArgs`/`AttachError`；判断 `IsError/From`；出口只出 Message 其余进日志；**错误编码不出响应体** |
| 错误多语言 | 每模块自己的 `messages_<语言>.go`（key 即 code，`{0}` 位置占位符）；`go generate`（i18nerr）产出构造函数；模块字典跟包走、无全局注册 |
| 删除确认 | 确认消息必须带实体名：模板 `{0}` + `InsertArgs(name)`（用户/角色/登录策略同理） |
| 输入转换 | 布尔输入为 int：1→true、其余（含 0）→false、零值即默认 false（`units.Bool`）；日期两种字符串 `20261021`/`20261021100000` 与 13 位毫秒时间戳经 `common/units` 解析为**本地时间**；DTO→service VO 用 `request.Wrap[VO]`，必填校验 `request.Require` |
| 请求上下文 | `rctx.RuntimeContext` **字段私有**（`WithID/WithTime/WithName/WithUser/WithLang` 链式赋值、`FromXxx` 取值）；中间件 `FillRuntimeContext()` 两路注入；**userName 即审计操作者**（原 actortx 包已并入删除，GORM 审计钩子自动填 CreatedBy/UpdatedBy） |
| 分页 | 请求 `paging.PageRequest`（pageIndex/pageSize，默认 **1/10**、上限 **MaxPageSize=100**）贯穿全链路；出参 `paging.Pageable` 含 **totalPage**（ceil，整除不加） |
| 公共库定位 | **只提供能力**，译文/配置由业务服务自维护；日志为完整工具（用户拍板）；**组件自治**：组件自带 `Init(m)`，是否启用由组件自己的配置段决定 |

## 3. 当前进度（快照日期 2026-10-21）

- **common-lib 核心能力改造完成并推送**（c894212）：
  - 新增 `common/config`（配置加载全套，覆盖率 100%）
  - `logx` 重构：Init + 包级函数 + lumberjack 双出口 + caller；删除远程
    出口（logsink）/自定义格式/自研滚动/NewSlog（覆盖率 100%）
  - `fverr` → `bizerror`（FvError → BizError 仅更名，生成器随迁零漂移）
  - `actortx` 并入 `rctx`（字段私有化 With/From）
  - 分页 Query → PageRequest、默认 10/上限 100、新增 totalPage
  - **Unreleased 已积累大量 breaking，接入业务前建议打 tag v0.6.0**
- **cmd/server 统一装配已落地**（中间件零持有日志 + server.NewEngine/
  Init/NewServer/Run + wire ProviderSet；security-management 已迁移 wire
  形态——main 只剩 InitializeServer → Run；详见 common-lib/HANDOVER 第
  6 节；存储组件 Init 形态仍待设计）
- **security-management**：用户模块纵切片完成并已随 common-lib 改造迁移
  （application.yml + LoadConfig/Init，端到端实测通过）；角色与登录策略未开始
- 已知遗留（不阻塞）：snowflake WIP（race + 3 条 lint + 覆盖率 + 同毫秒
  序列缺陷致 app 测试间歇失败）、repository 包无测试、txId 缺失行为待
  拍板（详见 common-lib/STATUS.md）

## 4. 下一步（按优先级）

1. **存储组件 Init 形态设计**（cmd/server 装配的遗留项）：oracle/CH/OS
   从 config map 初始化、enabled 判断方式
2. **角色模块 + 用户↔角色关联（事务场景）**——驱动解决 common-lib 差异
   点 #1（事务 context 约定，详见 security-management/README 的差异点）
3. common-lib 补齐实战暴露的缺口：通用仓储查询（Get/List/Delete/Count）、
   审计内存 helper、`NewPageable` 空值类型推断（差异点 #2/#4/#5；
   配置加载器已由 common/config 解决）
4. snowflake WIP 修复（含同毫秒序列缺陷——app 测试间歇失败的根因）
5. 集成线补跑（本轮大改后 Oracle/CH/OS 真环境）+ 打 tag v0.6.0
6. OpenSearch 用户搜索（domain 查询端口 + storage/opensearch 实现）
7. 认证中间件接入（api.WithActor 已改经 rctx 派生）

## 5. 验证命令

```bash
cd common-lib && make lint test        # 公共库（race 必含；snowflake WIP 除外）
cd security-management && go build ./... && go test ./...
cd security-management && go run ./cmd/server   # memory 模式自举（读 application.yml）
```
