# 调试日志、指标与 Trace 规范

## 1. 目标

运行时代码必须留下足够证据，让维护者能快速回答：请求在哪一步、基于什么状态做了什么决策、失败属于哪一类、是否发生重试或 Fallback、最终结果和耗时如何。

“多写调试日志”是多记录关键决策和状态转换，不是打印所有变量或请求正文。

## 2. 日志级别

- `Debug`：候选筛选、能力校验、缓存命中判断、重试等待、状态机转换、配置差异等诊断细节。
- `Info`：进程生命周期、配置成功生效、请求最终结果、成功完成的路由/Fallback。
- `Warn`：可恢复依赖故障、重试、熔断、明确进入的降级状态、客户端断流。
- `Error`：操作无法完成、持久化失败、状态不一致、不可恢复的请求或后台任务失败。

禁止同一错误在每层无差别重复记录。通常由能够补充完整业务上下文或决定最终结果的层记录。

## 3. 结构化字段

按场景尽量携带稳定字段：

- 请求：`request_id`、`trace_id`、`tenant_id`、`project_id`、`key_id`，禁止明文 key。
- 路由：`route`、`virtual_model`、`provider`、`model`、`required_capabilities`、`decision_reason`。
- 韧性：`attempt`、`max_attempts`、`error_kind`、`retry_after_ms`、`breaker_from`、`breaker_to`。
- 性能：`duration_ms`、`queue_ms`、`upstream_ms`、`first_token_ms`。
- 配置：`config_version`、`changed_sections`、`reload_result`、`restart_required`。
- 账务：`reservation_id`、`usage_event_id`、`estimated`、`settlement_result`，不得记录敏感余额主体信息。

字段命名保持稳定，不要在不同包交替使用 `req_id`、`requestId`、`request_id`。新代码统一采用 snake_case；迁移现有字段时避免一次产生无关大 diff。

## 4. 必须记录的决策点

- Provider/模型候选为什么进入或退出候选集。
- 缓存为何命中、未命中或被跳过。
- 重试是否发生以及错误分类、次数和等待时间。
- 熔断器状态变化和触发原因。
- Fallback 的原目标、失败类型、最终目标、能力检查和成本/延迟差异。
- 热重载的版本、差异、成功/拒绝以及哪些字段需要重启。
- 配额预占、结算、释放和冲正的状态转换。
- 后台清理、迁移、异步任务失败及受影响范围。

示例：

```go
logger.Debug("route candidate rejected",
    "request_id", requestID,
    "route", routeName,
    "provider", target.Provider,
    "model", target.Model,
    "decision_reason", "capability_mismatch",
    "required_capabilities", required,
)
```

反例：

```go
logger.Debug("request", "body", rawBody, "api_key", token)
```

## 5. 敏感信息与日志体积

永不记录：

- API Key、Authorization、Cookie、数据库密码、上游 Credential。
- 完整 Prompt、完整响应、Tool 参数原文和文件内容。
- 未脱敏的邮箱、手机号、身份证、银行卡等 PII。
- 可直接重放的 SSE 原始流；Fixture 必须先脱敏。

错误正文只能记录经过截断和脱敏的摘要。高频 Debug 日志应避免大对象、无界数组和热路径昂贵序列化；必要时使用采样或统计指标，但不能因此删除关键错误日志。

## 6. 日志、指标和 Trace 的分工

- 日志回答单次事件发生了什么。
- 指标回答问题影响了多少请求、持续多久、趋势如何。
- Trace 回答跨中间件、Router 和 Provider 的耗时与因果关系。

新增关键失败状态时至少考虑对应 counter；新增关键耗时阶段时考虑 histogram/span。Span 必须传播 `context.Context`，错误分类和选定 Provider 可作为属性，但不能放秘密或高基数字段。

## 7. 验证

- 测试关键错误路径能否产生包含必要上下文的日志。
- 检查日志不含测试 Token、Authorization 和请求正文。
- 验证同一个 `request_id`/Trace 能串联网关入口、路由决策和上游调用。
- 对降级和恢复分别验证日志、指标与健康状态。
