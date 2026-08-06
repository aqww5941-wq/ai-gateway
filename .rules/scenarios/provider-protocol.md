# Provider、Canonical IR 与厂商协议规则

## 1. 基本边界

协议链路必须拆为：

```text
Ingress Decoder -> Canonical Request -> Capability Planner
-> Provider Encoder -> Transport -> Provider Decoder
-> Canonical Response/Event -> Ingress Encoder
```

Provider Adapter 不负责租户策略、选路、重试、额度或 HTTP Handler。Transport 不理解 Canonical 语义；Codec 不直接发网络请求。

## 2. 禁止伪兼容

- 首批 Adapter 使用 `ark`、`deepseek`、`qwen` 独立包和版本，不以一个 `openai_compatible` 包加 Base URL/模型名区分。
- 字段同名不代表语义相同。厂商忽略参数、默认行为差异、模型限制和 region endpoint 都进入 Capability/Translation Report。
- 未知字段不得盲目透传；未知响应事件不得一律忽略。影响完成、Usage、Tool 或状态的未知事件返回 protocol error。
- 无法保真转换时在上游调用前拒绝，不删除 Tools、Reasoning、Structured Output、Multimodal、Usage 或 State。

## 3. Canonical IR 最小语义

IR 使用 typed Item/Content Block/Event，至少表达：文本、图片/文件引用、tool definition、tool call/result、reasoning、structured output 要求、usage、finish reason、provider state 和 vendor extension。ID、顺序、`call_id`、output/content index 和 reasoning state 必须稳定。

每次编码产生 Translation Report：`exact`、`normalized`、`lossy`、`unsupported`。`lossy` 是否允许由显式策略决定；`unsupported` 不执行。

## 4. 三家 Adapter 的硬约束

### 火山引擎方舟

- 首选 Ark Responses 语义，保留 typed output items、`call_id`、`previous_response_id`、thinking 和内置工具事件。
- Chat 与 Responses 是两个 dialect version，不能共享未验证字段集合。
- Endpoint、模型、region 与 thinking/tool 支持进入能力矩阵；只对官方文档和实测覆盖的组合标记 verified。

### DeepSeek

- 显式解析 `reasoning_content` 与 `content`，流式时保持二者顺序和事件类型。
- thinking 模式发生 tool call 后，后续请求必须完整回传该 assistant 消息的 `reasoning_content`；丢失会导致 400，Canonical State 和多轮编码必须覆盖此回归。
- JSON Output 是 `json_object` 语义，不冒充严格 JSON Schema；Prompt 约束和可能空内容进入文档/测试。
- strict tool mode 若使用 beta endpoint，必须作为独立 Capability/Endpoint 配置，不能默认为稳定生产能力。

### Qwen（阿里云百炼）

- Chat 与 Responses dialect 分开建模；Responses 的 `previous_response_id`、typed output 和内置工具不可降格为字符串。
- Chat thinking 使用模型支持的 `enable_thinking`/`reasoning_content`；Responses 依据当前文档使用 `reasoning.effort`，不混用参数。
- 流式 Usage 处理 `stream_options.include_usage`，不能假定每个 chunk 都有 Usage。
- 多模态、JSON mode、工具、仅流式模型和最大 Token 参数均按具体模型/region/API version 声明。
- Workspace 专属 Endpoint 与地域是配置的一部分；禁止把旧 URL 或单一全球 Base URL 写死。

## 5. SSE 与 Commit

- Parser 支持跨 buffer、CRLF、多 `data:` 行、Unicode、注释/heartbeat、流内 error、异常 EOF 和超大事件限制。
- 内部统一为 typed events，例如 started、text.delta、reasoning.delta、tool_call.arguments.delta、usage、completed、failed。
- 首个可写合法事件前允许按策略 Retry/Fallback；Commit 后只能在当前协议内显式完成或失败。
- 客户端写失败立即取消上游并进入确定的 Usage 结算路径，不继续发送终止标记。

## 6. 证据与测试

- 每项能力必须关联官方文档、脱敏 Golden/Replay 和 Adapter revision。
- Offline Conformance 不等于真实 Provider 验证。真实 Smoke 记录模型、region、endpoint、日期和结果；无 Key 标记 unverified。
- Fixture 禁止包含 API Key、Authorization、完整用户内容、PII 或可关联账户的 ID。
