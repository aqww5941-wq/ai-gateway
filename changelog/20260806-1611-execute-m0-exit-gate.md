# 执行 M0 可信基线 Exit Gate

- 日期：2026-08-06 16:11 CST
- 类型：L1
- 关联需求：Task 10
- 影响模块：`docs/gates/M0.md`、v3 任务状态、M0 验收流程

## 根因

Task 1～9 的证据分散在基线报告、测试、构建文档和 CI 中，没有证明它们能在同一个全新检出上同时成立，也没有一个明确决定把 M0 残余风险与 M1 放行绑定。直接开始 Gin 重构会无法区分新回归与被当前工作树掩盖的跨平台基线问题。

## 解决方案

在 detached Windows 全新工作树上执行两次完整构建、生成产物哈希/工作树检查、默认安全配置启动 Smoke、全仓 Test/Race/Vet/Staticcheck、前端质量和 Gitleaks。将首次 Golden EOL 失败、Task 10.R1 修复、复验结果、readiness 真实边界和所有残余风险集中记录到 `docs/gates/M0.md`。

Gate 采用两阶段状态：本地门禁通过但最新 HEAD 未有远端 CI 证据时保持 `In Progress`；只有 GitHub-hosted 三个 Job 全绿后才 Passed 并释放 Task 11。

## 行为与兼容性

本 Gate 不修改 Gateway API、配置、数据库或运行时行为。当前静态 `/admin/health` 不被包装为 readiness；双 Server 真实健康端点仍由 Task 15 实现。本地验收通过不会提前把 M0 或任何厂商能力标记为 Verified。

## 可观测性

无运行时日志、指标或 Trace 变化。Gate 使用进程状态、HTTP 状态、Git 差异、测试退出码和 CI Job 结论作为可审计证据，不记录 Credential 或业务内容。

## 验证

- `[通过]` 修复后 detached Windows worktree @ `e7e4fd8`，初始无 `bin/`/`node_modules/`。
- `[通过]` 连续两次 `go run ./cmd/build`；18 个嵌入产物哈希一致，工作树干净。
- `[通过]` 启动 Smoke：SPA 200、Metrics 200、无 admin 身份的静态 Health 403，退出后无残留监听。
- `[通过]` Format、全仓 Test、Vet、Staticcheck v0.7.0、Actionlint v1.7.12、WSL 全仓 Race。
- `[通过]` Frontend ESLint、6 个 Vitest、TypeScript、Vite Build。
- `[通过]` Gitleaks v8.27.2，无真实 Provider API 调用。
- `[通过]` GitHub-hosted [`Quality` #31084263262](https://github.com/aqww5941-wq/ai-gateway/actions/runs/31084263262) 在 Head `06a841d85efad1c2258c8c6213e48e66a16187b1` 上全绿：Go quality、Frontend quality、Secret scan 均为 `success`。
- `[结论]` Task 10 Done，M0 Passed，Task 11 Ready。

## 风险与回滚

残余风险已按 Task 15、17、19～36、37～56 映射，不影响开始 M1 的“可信迁移基线”，但会阻止生产或企业级声明。Gate 报告回滚只会移除集中证据，不改变运行时；Task 10.R1 不应随报告回滚，否则全新 Windows 测试会重新失败。用户已有的 v2 设计文档删除不属于本任务，不会暂存或提交。
