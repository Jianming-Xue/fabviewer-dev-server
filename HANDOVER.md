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
    └── 详见 security-management/README.md（分层结构/快速开始/差异点记录）
```

服务经 `replace github.com/fab-viewer/common-lib => ../common-lib` 引用
公共库（相对路径，挪根后仍成立）。新增业务服务照此并列。

## 2. 跨模块约定（2026-10-20 会话拍板，改动前必读）

| 约定 | 内容 |
|---|---|
| 请求运行时上下文 | 类型 `rctx.RuntimeContext`（五字段含 Lang）；填装中间件 `middleware.FillRuntimeContext()`；塞/取 `rctx.SetRuntimeContext/GetRuntimeContext` |
| 操作者通道 | 包 `actortx`，`SetActorContext/GetActorContext`；GORM 审计钩子自动填 CreatedBy/UpdatedBy |
| 统一错误 | `fverr.FvError` 四字段；唯一构造 `NewError(code,msg,args...)` + 链式 `InsertArgs`/`AttachError`；判断 `IsError/From`；出口只出 Message 其余进日志；**错误编码不出响应体** |
| 错误多语言 | 每模块自己的 `messages_<语言>.go`（key 即 code，`{0}` 位置占位符）；`go generate`（i18nerr）产出构造函数；模块字典跟包走、无全局注册 |
| 删除确认 | 确认消息必须带实体名：模板 `{0}` + `InsertArgs(name)`（用户/角色/登录策略同理） |
| 日志 | `logx.New(Config)`：zap 引擎，stdout/文件（编号滚动 log.N，上限可配 200）/OpenSearch 直写（logsink，异步批量绝不反压）可组合；每出口可注入自定义格式（LineEncoder） |
| 公共库定位 | **只提供能力**，译文/文案/服务配置由业务服务自维护；唯一例外：日志为完整工具（用户拍板） |

## 3. 当前进度（快照日期 2026-10-20）

- **common-lib**：i18n/FvError/命名对齐/logx 完整工具/logsink 均已完成并
  提交；CHANGELOG Unreleased 积累大量 breaking（错误体系/命名/日志），
  **建议在首个业务服务接入前打 tag（如 v0.6.0）锁定基线**
- **security-management**：用户模块纵切片完成（CRUD/i18n/删除确认带名/
  审计/memory 自举），端到端实测通过；角色与登录策略未开始
- 已知遗留（不阻塞）：snowflake 测试 race + 3 条 lint、repository 包无
  测试、txId 缺失行为待拍板（详见 common-lib/STATUS.md）

## 4. 下一步（按优先级）

1. **角色模块 + 用户↔角色关联（事务场景）**——驱动解决 common-lib 差异
   点 #1（事务 context 约定，详见 security-management/README 的 6 条差异点）
2. common-lib 补齐实战暴露的缺口：通用仓储查询（Get/List/Delete/Count）、
   配置加载器、审计内存 helper、`NewPageable` 空值推断
3. OpenSearch 用户搜索（domain 查询端口 + storage/opensearch 实现）
4. 认证中间件接入（api.WithActor 替换 userName 请求头约定）
5. 若远端有外部 CI：路径需改为 common-lib/ 子目录

## 5. 验证命令

```bash
cd common-lib && make lint test        # 公共库（race 必含）
cd security-management && go build ./... && go test ./...
cd security-management && go run ./cmd/server   # memory 模式自举，curl 示例见其 README
```
