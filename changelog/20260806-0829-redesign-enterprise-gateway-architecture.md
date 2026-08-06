# 重设企业级 AI Gateway 架构基线

- 日期：2026-08-06 08:29 CST
- 类型：L3
- 关联需求：N/A（新设计提案）
- 影响模块：`docs/AI_Gateway_v3_企业级重构设计文档.md`

## 根因

当前实现以狭窄的 OpenAI Chat 文本结构作为内部领域模型，Provider 同时承担协议转换、HTTP、流式解析与错误分类，导致所谓多厂商接入主要停留在 OpenAI-compatible Base URL 转发。HTTP Handler 又集中编排缓存、路由、韧性、Provider、用量和审计，使框架迁移、协议演进和企业不变量无法独立验证。原设计已由用户删除，不能继续作为实施基线。

## 解决方案

新增 v3 Proposed 设计，明确采用统一 Gin HTTP 入口、数据面/控制面双监听、框架无关的 Application/Domain 核心，以及 Ingress Codec、Canonical IR、Capability Planner、Native Adapter、Translation Report 和统一流事件。设计同时定义租户与密钥、额度预占和不可变账本、版本化运行时快照、PostgreSQL/Redis 边界、可观测性、测试门禁与 M0～M6 实施顺序。

首期厂商范围收敛为 OpenAI Responses/Chat、Anthropic Messages、Gemini GenerateContent 和经过合约测试的 OpenAI-compatible dialect，避免以厂商数量代替协议正确性。

## 行为与兼容性

本次只新增 Proposed 设计文档，不修改运行时代码、API、配置、Schema 或当前 README 行为。旧 v2 文档和场景规则的删除是用户已有修改，本次未恢复、未覆盖，也不会纳入提交。

## 可观测性

不适用；无运行时代码变化。新设计定义了后续稳定日志字段、指标、Trace Span、Route Dry Run 和 Request Timeline 要求。

## 验证

- `[通过]` 检查新文档章节、Mermaid、代码块和官方资料链接完整。
- `[通过]` 对照当前 `provider.ChatRequest`、OpenAI/Claude Provider、Server 调用链、Quota 和 Reload 实现复核根因。
- `[通过]` 使用 OpenAI、Anthropic、Gemini 与 Gin 官方文档核对协议和框架边界。
- `[未执行]` Go/前端测试；本次无运行时代码、配置或构建行为变化。

## 风险与回滚

设计仍为 Proposed，Gin 双平面、首批厂商范围和里程碑需要用户确认后才能成为实现基线。回滚只需删除新增设计与本 changelog，不涉及数据或运行时迁移。
