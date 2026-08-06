# 项目事实、方向与信息源

## 1. 产品目标

AI Gateway 为客户端提供稳定的 LLM API 入口，在内部统一处理协议转换、能力路由、上游凭据、额度、成本、审计和可观测性。

“统一”不是把所有厂商压缩成文本聊天最低公共子集。Tools、Reasoning、Structured Output、Multimodal、Streaming、Usage、厂商状态对象和错误语义必须被显式表达；无法无损转换时必须拒绝或返回可审计的 Translation Report。

## 2. 现状与目标必须分开

当前实现：

```text
Client -> net/http Server -> Auth/Cache/Router/Retry/Breaker -> Provider -> LLM
React Admin -> /admin/api/v1 -> Key/Quota/Route/Audit
```

目标架构：

```text
OpenAI-compatible Client Protocol
  -> Gin Data Plane / Gin Control Plane（双 Engine、双监听）
  -> Framework-independent Application Services
  -> Canonical IR + Capability Planner + Execution Plan
  -> Volcengine Ark / DeepSeek / Qwen Native Dialect Adapters
  -> Reservation + Usage Ledger + Audit + Observability
```

现有缓存、路由、重试、熔断、鉴权、配额和管理端是待评估迁移的资产，不因已存在就自动满足目标不变量。通过分阶段 Strangler Migration 迁移，避免无计划推倒重写。

## 3. 首批协议与厂商边界

- Ingress：OpenAI Chat Completions 与 Responses 兼容协议，用于接入现有 SDK 和 Agent 客户端。
- Egress 1：火山引擎方舟，优先覆盖 Responses、typed events、`previous_response_id`、`call_id`、thinking 和厂商内置工具语义。
- Egress 2：DeepSeek Chat Completions dialect，覆盖 `reasoning_content`、thinking + tool call 上下文回传、JSON Output、SSE 和 Usage。
- Egress 3：阿里云百炼 Qwen，覆盖 Chat Completions/Responses dialect、`enable_thinking`/`reasoning.effort`、typed Responses、模型相关多模态、工具和 Usage。
- OpenAI、Anthropic、Gemini 上游不在首批实测和交付范围；未来接入必须新增设计、Fixture 和真实验证，不得复用“兼容协议”暗示已支持。

厂商能力随模型、地域、Endpoint 和 API 版本变化。能力键至少使用 `provider + endpoint + region + model + protocol_version + adapter_revision`，不能只按厂商名声明。

## 4. 信息源优先级

1. 当前用户明确要求。
2. 根 `AGENTS.md` 与匹配的 `.rules/`。
3. `docs/AI_Gateway_v3_企业级重构设计文档.md` 的架构决策、里程碑和 Exit Gate。
4. ADR、OpenAPI/Schema、迁移说明和配置契约。
5. 对应厂商的当前官方 API 文档与已脱敏实测记录。
6. 测试表达的已确认行为。
7. 当前实现和 `README.md`。

当前代码只说明“现在如何运行”，不能证明“应该继续这样设计”。测试若固化了与新设计冲突的旧行为，应先说明契约变化，再同步更新。

## 5. 里程碑顺序

- M0 可信基线：可复现构建、严格配置、安全示例、关键回归测试和 CI。
- M1 Gin 入口与应用边界：双 Engine、GenerationService、标准错误和 Snapshot v1。
- M2 Canonical IR 与双 Ingress：Chat/Responses、typed events、Capability、Translation Report。
- M3 国内三厂商 Adapter：方舟、DeepSeek、Qwen 的独立 Codec、Conformance 与真实 Smoke Matrix。
- M4 能力路由与韧性状态机。
- M5 企业控制面、身份、Credential、预算与 Ledger。
- M6 多实例、故障注入、真实调用方和可复现性能报告。

必须满足当前里程碑 Exit Gate 才能把下一阶段作为主线。允许提前做研究或 Spike，但不得把未完成依赖包装成正式能力。

## 6. 当前优先风险

- 旧 `provider.ChatRequest` 由文本聊天子集定义，无法承载三家推理、工具、typed Responses 和多模态差异。
- `Server` 混合协议、策略、执行和持久化职责，无法可靠测试和热切换。
- 现有 Provider 抽象混合 Codec、Auth、Transport、SSE 和 Error Mapping，容易复制厂商分支。
- 能力声明缺少 `模型 × 地域 × Endpoint × 协议版本` 的实测证据。
- 配置、Quota 和热重载仍需通过 M0/M5 建立严格不变量。

不得继续在这些抽象上堆厂商字段或局部条件分支。
