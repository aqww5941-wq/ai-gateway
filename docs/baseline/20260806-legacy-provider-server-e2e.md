# 旧 Provider 与 Server 端到端回归基线

- 采集时间：2026-08-06 14:22 CST
- 代码基线：`master@2b21ec1`
- 关联任务：Task 7
- 网络边界：仅使用进程内 `httptest.Server`，不访问真实厂商
- Fixture 数据：仅使用 `fixture-*` 合成标识、模型、内容和无效凭据

## 1. 目的与范围

本文把旧 OpenAI-compatible Provider 和 `net/http` Server 的关键 HTTP 行为固定为迁移对照。后续 Gin 入口、Application Service、Canonical IR 和国内厂商 Adapter 重构时，可以判断 URL、Header、Body、错误状态、Retry/Fallback、SSE 顺序或 Context 传播是否发生非预期变化。

Task 7 只记录旧链路现状，不为旧 `provider.ChatRequest` 增加 Tools、Reasoning、Multimodal 或厂商字段，也不把离线 Fixture 宣称为真实 Provider 验证。

## 2. Provider 合约证据

| 场景 | 自动断言 | 当前结果 |
| --- | --- | --- |
| Unary 请求 | Method、Path、Content-Type、Authorization、Model、Messages、Temperature、MaxTokens | `POST {base_url}/chat/completions`，Bearer 凭据只进入 Header，请求字段按旧结构编码 |
| Unary 响应 | ID、Model、Choice、Usage | JSON 响应解码为旧 `ChatResponse` |
| HTTP 错误 | Unary 与 Stream 的 429、Retry-After、Provider、BodyLen、可重试分类 | 两条路径都返回 `*UpstreamError`，429 被标为 retryable，错误不包含 Provider Credential |
| SSE Replay | heartbeat、CRLF/LF、数据顺序、Usage、`[DONE]` 后数据 | 注释被忽略，两个合法 Chunk 按序输出，`[DONE]` 关闭 Channel，之后事件不再读取 |
| Stream 取消 | 首 Chunk 后取消 Context | 上游请求 Context 收到取消，Provider Channel 关闭 |

## 3. Server 全链路证据

测试通过真实 Gateway HTTP Handler，链路包含 Auth Middleware、模型权限、Router、Retry、Fallback、Provider HTTP Transport 和响应编码。

| 场景 | 自动断言 | 当前结果 |
| --- | --- | --- |
| Auth | 缺失或错误客户端凭据 | 返回 401，上游零调用 |
| Model Policy | Key 不允许请求模型 | 返回 403，上游零调用 |
| Route | 客户端请求 virtual model | 上游 Body 使用 route target model，客户端合成内容保持一致 |
| Unary 成功 | Ingress→Egress→Ingress | 返回上游 ID、模型、内容和 Usage；上游恰好调用一次 |
| Retry + Fallback | primary 连续 503，secondary 成功 | primary 按测试策略调用两次，之后 secondary 调用一次并返回成功 |
| 429 映射 | 单目标返回 429 | Gateway 返回 429，关闭重试时上游只调用一次 |
| SSE | 完整 Handler 下首事件、Usage 事件和结束标记 | Gateway 返回 `text/event-stream`，事件顺序稳定并以 `[DONE]` 结束 |
| 客户端取消 | 客户端读到首个 SSE 行后取消并关闭 Body | 取消传播到 Gateway 请求和上游 Stream Context |

## 4. 已证明或确认的旧链路缺口

| 证据 | 当前行为与根因 | 后续责任 Task |
| --- | --- | --- |
| `TestLegacyGatewayCurrentFallbackContinuesAfterUpstream400` | 旧 Fallback 对除 Context 外的全部错误继续下一个目标，上游 400 也会触发 Fallback；缺少分阶段稳定错误分类 | Task 37、40、42 |
| Retry/Fallback 代码与 Fixture | 旧 Chat POST 没有幂等证明，retryable HTTP 错误可直接重复请求 | Task 37、40 |
| `readSSEStream` 审计 | malformed JSON 和 Scanner 错误只写日志后关闭 Channel，调用方不能区分 `[DONE]`、异常 EOF 和协议损坏 | Task 17、22、30/32/34 |
| Stream 写路径审计 | 客户端写失败会取消上游，但当前 Handler 随后仍尝试写 `[DONE]`，且没有确定的部分 Usage 状态 | Task 17、52 |
| 旧请求结构 | `ChatRequest` 只能表达文本消息与少量采样字段 | Task 19～27 |

后续任务修复这些缺口时，应将“当前错误行为”断言改成新目标契约；不得为了维持刻画测试而保留错误语义。

## 5. 验证结果

- `[通过]` `go test -count=1 ./internal/provider ./internal/server`
- `[通过]` `go test -count=1 -cover ./internal/provider ./internal/server`：Provider 41.1%，Server 51.9%
- `[通过]` WSL `go test -race -count=1 ./internal/provider ./internal/server`
- `[通过]` `go test -count=1 ./...`、`go vet ./...`、`go build -o NUL ./cmd/gateway`
- `[通过]` Credential-shaped literal Secret Scan 与 `git diff --check`
- `[通过]` Fixture 网络审查：仅 `httptest.Server` 回环地址，无真实厂商 Endpoint
- `[通过]` Fixture 数据审查：无真实 API Key/Authorization、真实 Prompt/Response、PII 或账户 ID

## 6. Task 7 Exit 判定

- [x] Provider 有独立 Unary、HTTP Error、SSE 和取消合约夹具。
- [x] Server 有穿过真实 Middleware 与 Provider Transport 的端到端夹具。
- [x] URL、Header、Body、首事件、结束标记和错误分类均可断言。
- [x] Auth、Route、Retry、Fallback、429 和客户端取消路径均有自动证据。
- [x] 所有上游均为本地 Fixture，不使用任何真实 Credential。
- [x] 已知旧链路缺口已映射到后续责任 Task，没有扩大 Task 7 实现范围。

结论：Task 7 可完成；Task 8 可以进入 Ready。M0 尚未完成。
