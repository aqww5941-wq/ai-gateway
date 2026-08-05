# Provider、协议与流式规则

适用于 Provider、Canonical IR、厂商 Codec、能力模型、SSE 与协议错误映射。

## 1. 方向

- 外部 API DTO、Canonical IR、厂商 DTO、Codec、Transport 和策略分层。
- Canonical IR 不是最低公共子集；公共能力结构化表达，厂商特性通过受控 Vendor Extension 保存。
- 不继续向旧 `provider.ChatRequest` 添加厂商特有字段。新能力按 v2 M1/M2 进入 Canonical IR 和 Adapter。
- Provider 不同时拥有协议转换、Transport 策略、路由和业务计费职责。

## 2. 能力保持

Adapter 声明支持某能力，就必须通过对应合约测试：

- Text：角色、顺序、文本和 stop reason 不丢失。
- Streaming：事件顺序、增量拼接、结束和错误事件正确。
- Tools：定义、choice、call ID、名称、参数、结果和多轮历史完整。
- Reasoning：所需字段可正确传输并支持多轮回传。
- Structured Output：Schema 不被删除、弱化或改写。
- Multimodal：内容类型、顺序、媒体引用和限制得到验证。
- Usage：输入、输出、缓存、推理 Token 的含义明确。
- Error：429、5xx、超时、取消和协议错误正确分类。

无法保持请求语义时，只能选择兼容 Adapter/Model，或在上游调用前返回明确的 capability error。禁止删除不支持字段后继续请求。

## 3. SSE 与流式

- 使用脱敏 Fixture 和 Golden Test 验证分帧、跨 buffer 事件、空行、`[DONE]`、Unicode、错误帧和异常断流。
- 不假设一次 Scanner/Read 就得到完整 JSON 事件。
- 流发送首个客户端字节后，不跨厂商无痕重试；断流必须形成明确错误、审计与账务结果。
- 客户端取消应立即传播上游，不继续读取、缓存或计费为完整成功。
- 原始流不得进入普通日志；回放 Fixture 必须脱敏并控制大小。

## 4. Transport 与错误

- 连接池和超时属于共享 Transport 配置，不在每个 Adapter 复制。
- 保留上游状态码、受限错误摘要、Retry-After 和 provider error code，映射为稳定内部类型。
- 不重试确定性的请求校验和协议不兼容错误。
- 请求日志只能记录安全元数据，不记录 Header、Credential、完整请求或响应。

## 5. 验证

- 单元与 Golden Test 覆盖双向转换。
- 所有 Adapter 运行同一 Conformance Suite。
- 使用 mock upstream 验证 Header、URL、错误分类、取消和超时。
- 流式使用脱敏协议回放，验证首字节前后不同失败阶段。
