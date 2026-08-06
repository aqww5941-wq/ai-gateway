# 建立 Store、Key 与 Quota 保护测试

- 日期：2026-08-06 14:01 CST
- 类型：L1
- 关联需求：Task 6
- 影响模块：`internal/store`、`internal/middleware`、`docs/baseline`、`docs/AI_Gateway_v3_项目实施任务书.md`

## 根因

当前 Store 没有自动测试，Auth 测试使用共享语义容易混淆的内存 SQLite；Migration、Key 生命周期、Quota 并发和 Store 故障语义只能依靠代码阅读判断。尤其是 Quota 的请求前检查与请求后记账分离、月额度未参与判断、Auth 缓存延迟撤销和 Quota Store 故障 Fail Open，若不先建立证据，后续 Ledger 重构无法区分旧缺口与新回归。

## 解决方案

为 Store 和 Middleware 增加基于独立临时 SQLite 文件的保护测试，覆盖新库建表、旧库补列、重复 Migration、Key 摘要存储与完整生命周期、日/月 Usage 累计、关闭 Store 后的错误传播、Auth 故障以及 Quota 并发路径。并发测试使用屏障让 16 个请求先同时通过检查，再结算用量，从而确定性证明当前 check-then-record 会穿透额度。

新增 Task 6 基线报告，将版本化 Migration、即时 Key 撤销、月额度、原子预占、Fail Closed 和幂等结算缺口分别交给 Task 45、46、48、51、52。Task 6 更新为 Done，Task 7 更新为唯一 Ready 项。

## 行为与兼容性

没有修改运行时代码、API、配置或数据库 Schema。新增测试固定当前行为，其中已知缺口测试是后续目标契约的输入，不表示这些缺口已被修复。

## 可观测性

没有修改运行时日志、指标或 Trace。测试验证 Quota Store 故障日志具有诊断信息且不包含 Bearer Token。

## 验证

- `[通过]` `go test ./internal/store ./internal/middleware`。
- `[通过]` `go test -count=1 -cover ./internal/store ./internal/middleware`：Store 64.9%，Middleware 39.6%。
- `[通过]` WSL `go test -race ./internal/store ./internal/middleware`。
- `[通过]` `go test -count=1 ./...`。
- `[通过]` `go vet ./...`。
- `[通过]` `go build -o NUL ./cmd/gateway`。
- `[通过]` Credential-shaped literal Secret Scan。
- `[通过]` `git diff --check`。

## 风险与回滚

风险仅限测试对当前实现语义的约束。后续修复已知缺口时必须同步把相应刻画断言改为目标契约，不能为保留旧测试而维持错误行为。回滚可删除新增测试和基线报告并恢复任务状态，不涉及数据迁移。用户已有的 v2 设计文档删除不属于本任务，不会暂存或提交。
