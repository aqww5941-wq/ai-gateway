# 日志、指标、Trace 与诊断证据

## 1. 目标

每个请求都应能回答：使用哪个配置版本、需要什么能力、为何选择或拒绝目标、协议转换有何损失、是否 Retry/Fallback、何时 Commit、最终 Usage 如何结算。多写日志是记录关键决策，不是打印请求正文。

## 2. 稳定结构化字段

- 请求：`request_id`、`trace_id`、`tenant_id`、`project_id`、`key_id`。
- 协议：`ingress_protocol`、`egress_protocol`、`provider`、`endpoint_id`、`region`、`model`、`adapter_revision`。
- 规划：`virtual_model`、`required_capabilities`、`decision_reason`、`translation_warnings`、`config_revision`。
- 韧性：`attempt`、`max_attempts`、`error_kind`、`retry_after_ms`、`breaker_from`、`breaker_to`、`committed`。
- 性能：`duration_ms`、`queue_ms`、`upstream_ms`、`first_token_ms`。
- 账务：`reservation_id`、`usage_event_id`、`usage_estimated`、`settlement_result`。

字段统一 snake_case。Prometheus Label 不使用 request、tenant、key、response ID 等高基数值。

## 3. 级别与记录位置

- Debug：候选筛选、能力证明、转换警告、缓存判断、重试等待、Snapshot 差异和状态机转换。
- Info：进程生命周期、配置发布、请求最终结果、成功路由/Fallback。
- Warn：可恢复依赖错误、重试、断流、明确降级、缺失 Usage 的估算。
- Error：请求或后台操作无法完成、协议损坏、持久化失败和状态不一致。

同一错误不在每层重复打印；由能补全业务上下文或决定最终结果的层记录。底层返回带原因链和分类的错误。

## 4. 必须观察的状态转换

- Capability 推导以及每个候选的进入/拒绝原因。
- Adapter 编码产生的 Translation Report；未知字段/事件的处理策略。
- Provider connect、首个合法事件、Commit、完成、客户端取消和上游失败。
- Retry、Fallback、Breaker 进入/恢复和 Retry-After。
- Runtime Snapshot 构建、校验、发布、拒绝和旧资源回收。
- Quota reserve、settle、release、reversal、estimated 与 reconcile。
- Credential 解密失败、权限拒绝和 SSRF 防护命中，但不记录秘密内容。

关键失败状态至少考虑 counter；关键耗时阶段考虑 histogram/span。Span 使用 `context.Context` 传播，属性不得包含秘密或完整内容。

## 5. 敏感信息

永不记录 API Key、Authorization、Cookie、数据库/KMS 密码、Provider Credential、完整 Prompt/Response、Tool 参数原文、文件内容、未脱敏 PII 或可重放 SSE。上游错误正文只保留截断、脱敏摘要。Fixture 和真实 Smoke 报告先脱敏再落盘。

## 6. 验证

- 测试关键错误路径包含 request ID、provider/target、error kind、stage 和 committed。
- Secret Scan 验证日志与 Fixture 不含 Token、Authorization 或正文。
- 验证同一 Trace 串联 ingress、plan、quota、provider、stream 和 settle。
- 对降级与恢复、Snapshot 接受与拒绝分别验证日志、指标和健康状态。
