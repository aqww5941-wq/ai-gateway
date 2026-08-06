# 建立重构前当前行为基线

- 日期：2026-08-06 09:16 CST
- 类型：L1
- 关联需求：Task 1
- 影响模块：`docs/baseline/20260806-current-state.md`、`docs/AI_Gateway_v3_项目实施任务书.md`

## 根因

项目现有测试、README 和代码分别描述了一部分能力，但没有统一记录工具链、路由、配置、Reload、Store、Provider、SSE、失败语义和覆盖缺口。若直接进入构建或架构重构，后续差异无法可靠区分为旧行为、新回归或目标契约变化。

## 解决方案

基于 `master@d058f55` 采集无缓存 Go Test/Coverage、Vet、Build、Race 和 TypeScript 证据，并逐项审计进程入口、HTTP 路由、Middleware、Chat/SSE、Provider、Config/Reload、Store/Auth/Quota/Audit、构建产物与测试分布。新增版本化当前行为报告，将已知失败语义映射到后续责任 Task。

Task 1 状态更新为 Done，Task 2 更新为唯一 Ready 项。

## 行为与兼容性

无运行时代码、API、配置、数据库或构建行为变化。报告只描述当前实现，不把 v3 Planned 能力写成 Implemented。

## 可观测性

不适用，无运行时代码变化。报告记录了当前日志、指标和 Trace 缺少统一 request/config/commit/adapter 字段的事实。

## 验证

- `[通过]` `go test -count=1 -cover ./...`，所有现有测试通过，总语句覆盖率 35.7%。
- `[通过]` `go vet ./...`。
- `[通过]` `go build -o $env:TEMP/ai-gateway-task1-gateway.exe ./cmd/gateway`。
- `[通过]` `npm exec tsc -- --noEmit`。
- `[通过]` Baseline 必需章节、Task 状态、疑似 Secret 和 `git diff --check` 校验。
- `[失败]` `go test -race -count=1 ./...`：当前 `CGO_ENABLED=0` 且无 gcc，Go 明确返回 `-race requires cgo`；未描述为通过，交由 Task 8/M0 Gate 建立门禁。
- `[未执行]` Vite build；当前会写入双份受跟踪产物，Task 2 负责先统一生成源。
- `[未执行]` 真实 Provider/启动 Smoke；Task 1 不使用外部 Credential，也不改变默认配置。

## 风险与回滚

报告中的失败语义来自当前代码审计，部分关键包尚无回归测试；Task 6、7 将把它们转为自动证据。回滚只需删除本次报告并恢复任务状态，不涉及运行时或数据迁移。用户此前删除的 v2 文档不纳入本任务提交。
