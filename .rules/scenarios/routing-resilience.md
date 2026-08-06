# 能力路由、Retry、Breaker 与 Fallback

## 1. Planner 顺序

```text
协议与所需能力
-> 租户/项目/Key 权限
-> 地域与合规
-> 预算/最大成本
-> Endpoint/Model 健康
-> 成本、延迟、质量和权重
```

Planner 输出不可变 ExecutionPlan，包含 Snapshot revision、首选与 Fallback、每个目标的能力证明、预算上界和拒绝原因。Provider 调用期间不重新读取全局配置。

## 2. 能力与厂商状态

- 能力按 provider、endpoint、region、model、protocol version 和 adapter revision 声明。
- `verified`、`documented`、`experimental`、`unsupported`、`unverified` 必须区分；只有 Mock 的能力不能进入真实生产候选。
- Tools、Reasoning、Structured Output、Multimodal、Streaming、Usage 和 Provider-managed State 分别判断，不能用 `supports_chat=true` 代替。

## 3. Retry 与 Fallback

- 只重试稳定分类为可重试且 Context 未取消的错误，遵守 Retry-After、总 Deadline、次数和等待预算。
- POST 若可能已被上游接收，只有幂等键受支持或能证明请求未写出时才重试。
- Fallback 重新检查 Capability、Policy、Region 和 Budget；更贵目标先原子调整 Reservation。
- 流式 Commit 后禁止切换厂商。取消、invalid request、capability mismatch 和权限错误不重试。
- Hedging 默认关闭；开启需显式成本预算、取消证明、幂等语义和重复结算防护。

## 4. Circuit Breaker

- 隔离键至少为 `endpoint_id + model + region`，必要时加 protocol version。
- 客户端 4xx、能力拒绝和取消不计上游故障；429、5xx、timeout、transport 和 protocol 按策略分别计权。
- Half-open 有并发上限；状态变化有日志、指标、事件和恢复测试。
- 本地 Breaker 保护实例；集群健康摘要用于规划参考，不以 Redis 取代本地保护。

## 5. 验证

覆盖 429/Retry-After、5xx、连接失败、读超时、取消、Commit 前后、能力不等价、预算调整、Half-open 并发和状态恢复。断言最终调用次数、选择原因、费用状态和诊断字段，而不只断言 HTTP status。
