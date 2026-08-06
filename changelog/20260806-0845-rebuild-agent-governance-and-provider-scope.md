# 重建 Agent 规则并收敛国内三厂商范围

- 日期：2026-08-06 08:45 CST
- 类型：L1
- 关联需求：N/A
- 影响模块：`AGENTS.md`、`.rules/**`、`docs/AI_Gateway_v3_企业级重构设计文档.md`

## 根因

原有规则仍引用已删除的 v2 设计，并把“当前 net/http 实现”固化为目标边界，与 v3 的 Gin 双入口和核心框架无关设计冲突。Provider 规划又把国外厂商和通用 OpenAI-compatible 转发作为首批目标，没有区分客户端协议兼容、厂商 Native Dialect 和真实 Credential 验证，无法保证首批能力可实际验收。

## 解决方案

重建根 `AGENTS.md`、四份核心规则和八份场景规则，统一现状/目标、M0～M6、Gin 边界、Canonical IR、能力路由、厂商证据、Snapshot、账本、可观测和原子交付约束。新增 `http-gin.md`，将框架入口、SSE Commit 和双平面生命周期单独治理。

v3 设计将首批上游收敛为火山引擎方舟、DeepSeek、阿里云百炼 Qwen。三家使用独立 Adapter 与 Capability Evidence，明确方舟 Responses 状态对象、DeepSeek reasoning 回传以及 Qwen Chat/Responses、地域 Endpoint 和模型差异。真实 API 测试通过环境变量 opt-in；无 Credential 的能力标记为 unverified。

## 行为与兼容性

本次只修改工程治理与设计，不改变运行时 API、配置、数据库和当前代码行为。OpenAI Chat Completions/Responses 仍作为客户端兼容协议；OpenAI、Anthropic、Gemini 不再属于首批上游交付和验证范围。

## 可观测性

无运行时代码变化。规则新增 `ingress_protocol`、`egress_protocol`、`endpoint_id`、`region`、`adapter_revision`、`translation_warnings` 和 `committed` 等后续实现必须携带的诊断字段。

## 验证

- `[通过]` 官方一手文档核对方舟 Responses、DeepSeek Thinking/Tool/JSON Output、Qwen Chat/Responses/Thinking/Streaming 差异。
- `[通过]` 根规则场景索引目标文件存在性检查。
- `[通过]` 旧 v2、国外首批 Native Adapter 和 Gin 旧边界残留文本扫描。
- `[通过]` `git diff --check`。
- `[未执行]` Go/前端测试；本次没有运行时代码或构建配置变化。
- `[未执行]` 三家真实 API Smoke；本次只定义验证契约，尚未实现 Adapter 与测试入口。

## 风险与回滚

风险是官方 API 能力会随模型、地域和版本变化，因此规则禁止仅按厂商名声明能力，并要求真实 Smoke 记录完整证据。回滚可恢复本提交中的治理和 v3 文档；不涉及数据迁移。用户此前删除的 v2 文档不纳入本任务提交。
