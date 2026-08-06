# Store、安全、租户、额度与账本

## 1. 租户与授权

- Organization -> Project -> Gateway Key/Policy/Budget；所有事实表和查询必须有租户边界。
- Repository 方法显式接收 tenant/project scope，禁止先查全局 ID 再由调用方补授权。
- 跨租户反向测试必须覆盖读、写、列表、导出、审计和缓存键。
- 控制面使用 OIDC/Session + RBAC；本地 bootstrap admin 只允许显式开发模式。

## 2. Key 与 Provider Credential

- Gateway Key 使用可定位 ID + 高熵 secret；数据库只保存前缀和 HMAC 摘要，常数时间比较。
- Provider Credential 使用 envelope encryption，存 credential reference、key version 和 rotation state；生产主密钥来自 KMS/Secret Manager。
- 禁止在配置、数据库、日志、错误、管理 API、Fixture 或 changelog 输出明文 Credential。
- Provider Base URL 来自 allowlisted Endpoint；默认禁 Redirect，防止 SSRF 和凭据转发到恶意主机。

## 3. Quota 与 Usage Ledger

状态至少为 `Requested -> Reserved -> InFlight -> Settled/PartiallySettled/Released/NeedsReconcile`。

- 请求前原子预占估算 Token/金额，完成后按 Usage 结算并释放差额。
- request/event ID 保证幂等；Ledger 只追加，修正使用 reversal/adjustment。
- Token/金额使用整数或 Decimal；保存请求时 Price Snapshot。
- 取消、断流、缺 Usage、超时和重复回调有确定状态；估算必须标记 `estimated=true`。
- Store 故障默认 Fail Closed；不得先查询后累加造成并发穿透。

## 4. 数据库与迁移

- 生产事实源为 PostgreSQL；SQLite 只用于 standalone，并通过 Repository Conformance 保持核心语义。
- Schema 变更使用版本化 migration，包含升级、兼容窗口、回滚/前滚策略和大表影响。
- 事务边界围绕业务不变量，不把网络 Provider 调用放进数据库事务。

## 5. 验证

覆盖并发预占、重复结算、释放/冲正、Store 故障、Key 禁用/轮换、Credential 解密失败、跨租户访问、SSRF/Redirect 和日志 Secret Scan。安全失败不得只测 happy path 状态码。
