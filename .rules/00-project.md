# 项目介绍与方向

## 1. 产品目标

AI Gateway 为个人开发者和企业团队提供稳定的 LLM 接入入口，在不修改客户端 Base URL 和 SDK 初始化方式的前提下，统一处理多厂商协议、能力路由、密钥、配额、成本、审计与可观测性。

“统一”不等于最低公共子集。Tools、Reasoning、Structured Output、Multimodal、Streaming、Usage 和厂商扩展必须能被表达、验证和正确转换；无法保持语义时应明确拒绝或选择兼容目标。

## 2. 当前架构

```text
Client
  -> Go net/http 数据面
  -> Auth / Quota / Rate Limit / Filter / Cache
  -> Router / Retry / Circuit Breaker / Singleflight
  -> Provider
  -> Upstream LLM

React + TypeScript 管理端
  -> /admin/api/v1
  -> Key / Quota / Route / Cache / Breaker / Audit
```

主要目录：

- `cmd/gateway/`：进程入口和组件组装。
- `config/`：YAML 配置、环境变量展开和热重载。
- `internal/server/`：HTTP 数据面及现有管理 API。
- `internal/provider/`：当前 Provider 协议与传输实现。
- `internal/router/`：权重、轮询、语义、延迟与 Fallback 路由。
- `internal/store/`：SQLite Key、Quota、Audit 和迁移。
- `internal/cache|retry|breaker|limiter/`：韧性与性能组件。
- `internal/observer|metrics|tracing/`：日志、指标和链路追踪。
- `web/src/`：管理端源代码。
- `web/dist/`、`internal/static/dist/`：当前存在的重复构建产物，不是手工编辑源。

## 3. 信息源优先级

处理项目问题时按以下顺序判断：

1. 当前用户明确要求。
2. 根 `AGENTS.md` 和匹配的 `.rules/`。
3. `docs/AI_Gateway_v2_重构升级设计文档.md` 的 Approved Baseline、需求编号和 Exit Gate。
4. ADR、迁移说明和 API 契约。
5. 测试所表达的已确认行为。
6. 当前实现和 `README.md`。

当前代码只能说明“现在怎么做”，不能自动证明“应该继续这么做”。测试可能固化旧行为；发现测试与新设计冲突时，必须解释并同步更新，不能为了让测试变绿而保留错误设计。

## 4. 演进边界

- 按 M0 至 M6 顺序推进，优先解决构建、配置、快照、迁移、并发额度和测试地基。
- 保护已有缓存、熔断、重试、路由和管理后台资产，通过分层迁移替代无计划重写。
- 数据面保留 `net/http` 对 SSE、连接生命周期和高并发的直接控制。
- Gin 仅用于规划中的控制面，不在新旧控制面之间复制业务规则。
- API DTO、Canonical IR、Provider Adapter、Transport 和业务策略逐步分离。
- SQLite 保留 standalone 模式；集群模式使用具备一致性保证的共享存储和状态组件。
- Token 管理目标是预授权、结算、释放、冲正、幂等事件和不可变账本，不继续扩展“请求前查询、请求后累加”的非原子模型。

## 5. 当前已知高风险基线

开始相关任务前，应在 v2 文档中读取对应编号：

- `BASE-001`：前端构建产物双份提交，构建不可复现。
- `BASE-002`：热重载不是完整原子快照。
- `BASE-003`：配置契约、默认值和校验不完整。
- `SEC-001`：示例配置包含固定可用 Key。
- `KEY-000`：Key 查询与失效机制不可扩展。
- `TOK-000`：配额检查与记账分离且错误时 Fail Open。
- `TOK-001`：当前用量记录不构成账本。
- `TEST-001`：Store、Provider、Config 等关键路径保护不足。

不得在这些问题上增加新的局部补丁。修改必须朝已批准的目标模型收敛。
