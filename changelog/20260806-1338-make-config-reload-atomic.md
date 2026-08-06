# 固定配置 Reload 原子发布边界

- 日期：2026-08-06 13:38 CST
- 类型：L2
- 关联需求：Task 5
- 影响模块：`config/reloader.go`、`config/runtime.go`、`internal/server/runtime.go`、Server 请求链路、管理 API、Metrics 与配置文档

## 根因

旧 `Reloader` 在 Reload callback 成功前就更新自己的 Config 指针；`Server.Reload` 只在一个锁内替换 `config`、`router`、`providers`，但请求会在不同阶段无锁读取这些字段。并发切换时，请求可能使用新 Router 查找旧 Provider。Auth、Quota、RateLimit、Cache、Filter、Transport、Store、Tracer 和 HTTP Server 均在启动时构造，却没有随 `s.config` 更新，导致管理 API 显示新配置而真实行为仍是旧资源的伪成功。

## 解决方案

- 建立带单调 revision 的不可变 M0 Runtime Snapshot，固定 Config、Router、Provider Registry、Breaker 和 Latency 状态，通过 `atomic.Pointer` 一次发布。
- 每个 Unary/Stream 请求只在入口捕获一次 Snapshot，并将其显式传入 Route、Fallback、Provider、Breaker 和 Latency 路径。
- Cache namespace 与 Singleflight key 加入 revision，避免路由变化后复用旧 revision 的响应或上游执行。
- 当前只允许 `providers`、`routes` 动态更新；`server`、`auth`、`quota`、`rate_limit`、`cache`、`filter`、`tracing` 变化返回可分类的 `RestartRequiredError`。
- Reload 串行构建候选：Clone、Validate、restart-required 对比、资源构建全部成功后才发布；失败和无变化均保留旧 Snapshot/revision。
- Route-only 变更复用未变化的 Provider/Breaker；Provider 变更重建 Provider、Breaker 和关联 Router/Latency 状态。
- `Reloader.Config()` 返回防御性副本，并且只有 callback 成功后才更新已接受配置。

## 行为与兼容性

Provider/Route 合法变更仍可不停机发布，但请求不再观察混合版本。启动期资源字段的热更新从过去的伪成功改为明确拒绝并要求重启。管理 API 的 Overview、Provider、Route、Latency、Breaker、Cache、Filter 和遗留 Route/Cache 响应新增 `config_revision` 字段。每次成功动态发布会为 Cache 使用新的 revision namespace；旧条目按现有容量/TTL 淘汰。

## 可观测性

Reload 日志记录 `from_revision`、`candidate_revision`/`config_revision`、`stage`、`changed_sections` 和结果，不记录配置正文或 Credential。新增 `gateway_config_reload_total{result,stage}`，Label 仅使用有限枚举，不把 revision 作为高基数 Label。

## 验证

- `[通过]` `C:\Program Files\Go\bin\go.exe test ./...`
- `[通过]` `C:\Program Files\Go\bin\go.exe vet ./...`
- `[通过]` `C:\Program Files\Go\bin\go.exe build -o NUL ./cmd/gateway`
- `[通过]` WSL Ubuntu：`go test -race ./config ./internal/server`
- `[通过]` Reload 成功、no-op、校验失败、restart-required、Snapshot 构建失败、候选后续修改、资源复用与 8×40 并发请求/40 次交替发布测试。
- `[通过]` 工作树高熵 Credential assignment 与常见 Token 前缀扫描无匹配。

## 风险与回滚

当前 Snapshot 不负责 Transport、Store 等带 Close 生命周期资源的引用计数回收，该能力按任务书留给 Task 16；M0 通过 restart-required 禁止这些资源热更新。频繁 revision 会暂时保留旧 revision Cache 条目，但受现有容量和 TTL 限制。回滚本提交会恢复部分发布和并发混用风险，无数据迁移。
