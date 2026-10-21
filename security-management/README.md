# security-management

安全管理服务（用户 / 角色 / 登录策略——本切片实现用户模块），基于
[common-lib](../common-lib) 构建，同时作为 common-lib 的实战检验场。

## 分层结构

```
security-management/
├── cmd/server/      组装根：装配日志、存储、服务、路由，优雅停机
├── api/             接口层：HTTP 路由与 handler（只认识 services 接口）
├── services/        接口定义：对外能力接口 + DTO（契约层，无实现）
├── app/             实现：services 接口的实现（业务语义、错误表达）
├── domain/          实体与端口：User 实体、UserRepository/TxManager 接口
├── storage/
│   ├── oracle/      Oracle 实现：PO 映射、GORM 查询、事务（生产）
│   └── memory/      内存实现：开发自举与测试（无库启动全链路）
└── usererrors/      用户模块错误语言包（i18nerr 生成构造函数）
```

依赖方向（箭头指向被依赖方）：`api → services ← app → domain ← storage`，
`app/storage → usererrors/common-lib`。domain 不依赖任何技术设施。

## 快速开始

```bash
go run ./cmd/server    # 无数据库环境变量 → memory 模式（重启即失）
```

Oracle 模式（生产）：设 `SM_DB_USER / SM_DB_PASSWORD / SM_DB_HOST /
SM_DB_PORT / SM_DB_SERVICE / SM_ADDR`，并执行 `storage/oracle/schema.sql`。

日志与配置（2026-10-21 起读仓库根 `application.yml`，appName=security）：
console（console 格式）+ `systemlogs/security-management.log`（json 格式，
lumberjack 滚动 100MB/200 份，caller 指向业务调用行）；叶子配置可经
`security.*` 环境变量覆盖（如 `security.log.file.maxSize=200`），文件路径
可经 `security_CFG` 覆盖。

```bash
curl -i -X POST -H 'Content-Type: application/json' -H 'userName: ops-admin' \
     -d '{"username":"zhangsan","name":"张三"}' http://localhost:8081/api/users
curl -i 'http://localhost:8081/api/users?pageIndex=1&pageSize=10'
curl -i -X DELETE -H 'Accept-Language: zh-CN' http://localhost:8081/api/users/<id>
# → 409 "确认删除用户 张三 吗？"（确认消息带姓名）
curl -i -X DELETE 'http://localhost:8081/api/users/<id>?confirm=true'   # → 204
```

## 已发现的 common-lib 差异点（实战检验记录）

1. **事务跨层传递缺约定**：`domain.TxManager` 已定义，但 GORM 事务句柄
   如何经 context 传给事务内的仓储调用没有现成方案（common-lib 可补
   `conn.NewTxContext/FromTxContext`）。多写场景接入前需解决。
2. **通用仓储只有 Create/Update**：查询/删除全走 GORM 出口自写——业务
   实际主要是查询，common-lib 的 `repository.Repository[T]` 可考虑补
   Get/List/Delete/Count。
3. ~~**配置无加载器**~~ **已解决**（2026-10-21）：`common/config` 落地，
   本服务已切到 application.yml + `LoadConfig`；存储段（oracle 等）待
   cmd/server 统一装配时一并切入。
4. **`paging.NewPageable` 的 nil 无法推断类型参数**：空结果必须显式
   `NewPageable[T](...)`。
5. **审计行为两套实现要各自保证**：oracle 走 BasePO 钩子自动填，
   memory 手填 `rctx` 操作者——容易漂移，common-lib 可提供内存场景 helper。
6. **模块码→状态码映射表**每服务手写（可考虑生成器顺带产出）。

## 路线图

- [ ] 角色 / 登录策略模块（roleerrors / policyerrors 各自语言包）
- [ ] 用户↔角色关联 + 事务场景（驱动 common-lib 补事务 context 约定）
- [ ] OpenSearch 用户搜索（domain 查询端口 + storage/opensearch 实现）
- [ ] 认证中间件接入（api.WithActor，替换 userName 请求头约定）
