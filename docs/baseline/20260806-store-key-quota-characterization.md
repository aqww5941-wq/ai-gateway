# Store、Key 与 Quota 当前行为保护基线

- 采集时间：2026-08-06 14:01 CST
- 代码基线：`master@3ec04b9`
- 关联任务：Task 6
- 数据库：每个测试独享 `t.TempDir()` 下的 SQLite 文件

## 1. 目的与边界

本文把当前 SQLite Store、Migration、Gateway Key、Auth Cache 和 Quota 的实际语义转成可重复测试，为后续 Repository、Key 与 Ledger 重构提供对照。测试只刻画事实，不把已知缺口伪装成企业级能力，也不在 Task 6 提前实现 PostgreSQL、租户模型或 Reservation/Ledger 状态机。

## 2. 已固定的当前语义

| 边界 | 自动证据 | 当前结果 |
| --- | --- | --- |
| SQLite 隔离 | Task 6 新增的 Store/Middleware 保护测试使用独立临时文件 | 不读取或污染开发数据库 |
| 初始 Schema | 检查 Key、Quota、Audit 表及索引 | 新库可建立当前 Schema |
| 重复迁移 | 同一 Store 重复执行 `migrate()` 并校验既有 Key | 当前建表与补列流程可重复执行且保留数据 |
| 旧库补列 | 从缺少 `role/models` 的 Key 表启动 | 自动补列并保留旧行，名为 `admin` 的 Key 获得 admin 角色 |
| Key 落库 | 检查 Token 格式和数据库字段 | 明文只在创建时返回，当前库保存 32 字节 SHA-256 摘要 |
| Key 生命周期 | 创建、查询、更新、禁用、启用、删除 | Store 查询会拒绝 inactive Key；删除 Key 级联删除 Quota 行 |
| Auth 故障 | 关闭 Store 后执行未缓存 Token 验证 | Auth 返回 500，不调用下游 |
| Usage 记账 | 连续写入并读取日/月快照 | 每次写入同时累计当天和当月 Token |
| Store 故障 | 关闭连接后调用 Key/Quota/Audit 方法 | 方法向调用方返回错误，夹具不吞错 |

## 3. 已证明的缺口

| 证据 | 当前行为 | 根因 | 责任任务 |
| --- | --- | --- | --- |
| `TestOpenCreatesSchemaAndMigrationIsIdempotent` | `PRAGMA user_version` 为 0 | Migration 没有版本、前滚记录和失败恢复契约；补列错误被忽略 | Task 46 |
| `TestAuthCurrentCacheDelaysRevocationUntilRefresh` | 已缓存 Key 在 Store 禁用后仍可调用，手工刷新后才返回 401 | Auth 以完整 Token 为缓存键，撤销不主动失效缓存 | Task 48 |
| `TestCheckQuotaCurrentBaselineDoesNotEnforceMonthlyLimit` | 月额度已用尽时仍允许请求 | `CheckQuota` 只读取并比较 daily limit | Task 51 |
| `TestQuotaCheckCurrentBaselineAllowsConcurrentBudgetOvershoot` | 日额度为 1 时，16 个并发请求均能在记账前进入下游，完成后用量为 16 | Quota 是“请求前查询、请求后记账”的非原子 check-then-record | Task 51 |
| `TestQuotaCheckCurrentBaselineFailsOpenOnStoreError` | Quota 查询失败时继续调用下游 | Middleware 显式记录错误后 Fail Open | Task 45、Task 51 |
| 当前 Usage 路径审计 | 请求完成后才写累计值，没有 request/event 幂等键、预占、释放或冲正 | 当前表只是周期累计器，不是预算与账本状态机 | Task 51、Task 52 |

这些测试会随责任任务迁移为目标契约测试：Task 46 引入版本化 Migration；Task 48 要求撤销即时生效；Task 51 要求原子预占且 Store 故障 Fail Closed；Task 52 负责最终结算、幂等和对账。

## 4. 验证结果

- `[通过]` `go test ./internal/store ./internal/middleware`
- `[通过]` `go test -count=1 -cover ./internal/store ./internal/middleware`：Store 64.9%，Middleware 39.6%
- `[通过]` WSL `go test -race ./internal/store ./internal/middleware`
- `[通过]` `go test -count=1 ./...`、`go vet ./...`、`go build -o NUL ./cmd/gateway`
- `[通过]` Credential-shaped literal Secret Scan 与 `git diff --check`
- `[通过]` Task 6 测试数据库路径审查：仅使用测试临时目录，不使用 `data/gateway.db` 或共享连接
- `[通过]` 日志断言：Quota Store 故障日志包含诊断信息且不包含 Bearer Token

## 5. Task 6 Exit 判定

- [x] 临时 SQLite Repository 测试不共享开发数据库。
- [x] 新库、旧库补列和重复 Migration 均有自动证据。
- [x] Key 创建、存储、查找、更新、启停和删除均有生命周期测试。
- [x] 并发测试确定性证明当前额度穿透，而不是依赖偶发调度。
- [x] Auth、Quota 和底层 Store 故障行为均有保护测试。
- [x] 已知缺口已映射到 Task 45、46、48、51、52，没有在当前任务扩大实现范围。

结论：Task 6 可完成；Task 7 可以进入 Ready。M0 尚未完成。
