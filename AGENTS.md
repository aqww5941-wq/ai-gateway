# AI Gateway Agent 工作入口

本文件是所有 Agent 进入仓库后的第一入口。它负责项目引导和规则索引；详细规范位于 `.rules/`。

## 1. 项目方向

- 本项目是多厂商 LLM 统一接入网关。当前实现包含 Go `net/http` 数据面、React 管理端、路由、Provider、缓存、重试、熔断、鉴权、配额、审计与可观测能力。
- `README.md` 描述当前可运行能力；`docs/AI_Gateway_v2_重构升级设计文档.md` 是后续开发的 Approved Baseline。两者冲突时，不得自行猜测；先依据 v2 设计和当前代码分析影响，必要时先更新设计文档。
- 当前演进顺序遵循 v2 文档的 M0 至 M6。不得绕过正确性、配置契约、测试和构建治理，直接堆叠后续功能。
- 数据面继续使用 `net/http`；规划中的 Gin 只用于控制面。不得借重构扩大框架迁移范围。
- 新协议能力进入 Canonical IR 和 Adapter 边界，不得继续把厂商字段堆入旧 `provider.ChatRequest`。

每次开始工作必须先阅读 `.rules/00-project.md`。修改文件时，还必须阅读 `.rules/01-engineering-principles.md`、`.rules/02-delivery-workflow.md`，运行时代码必须阅读 `.rules/03-observability.md`。

## 2. 强制工作流

1. 执行 `git status --short`，识别并保护用户已有修改。
2. 根据下方索引读取匹配的场景规则、相关设计、代码和测试。
3. 修改前写清：现象、根因、影响范围、设计方案、验证方式。禁止在未定位根因时止血式修补。
4. 实现完整方案，同步测试、调试日志、指标、Trace、配置、文档或迁移。
5. 按变更风险执行验证；不得把未执行的检查写成已通过。
6. 代码、配置契约、数据库、API 或测试行为有变化时，在 `changelog/` 新建一份变更记录。
7. 检查工作树和暂存区，只暂存本任务文件，完成一个原子 Git 提交。默认不 push。

只做阅读、分析或代码审查时，不生成 changelog，也不提交 Git。

## 3. 场景规则索引

| 场景或路径 | 必读规则 |
|---|---|
| `cmd/**`、`internal/**`、`config/**` 的 Go 代码 | `.rules/scenarios/go-backend.md` |
| `web/**` | `.rules/scenarios/web-frontend.md` |
| `internal/provider/**`、Canonical IR、厂商 Codec、SSE | `.rules/scenarios/provider-protocol.md` |
| `internal/router/**`、`retry/**`、`breaker/**`、故障转移 | `.rules/scenarios/routing-resilience.md` |
| `config/**`、构造默认值、`Server.Reload`、运行时状态 | `.rules/scenarios/config-runtime.md` |
| `internal/store/**`、鉴权、Key、Quota、Ledger、租户 | `.rules/scenarios/storage-security.md` |
| API、Schema、公共接口、兼容性、架构决策、文档 | `.rules/scenarios/docs-adr.md` |

同一修改命中多个场景时，相关规则必须全部读取。

## 4. 不可绕过的原则

- 修改前分析根因，禁止止血式修补。
- 不得吞错、伪造成功或把验证失败描述成环境问题。
- 不得为了“稳定”静默删除字段、关闭校验、绕过鉴权/配额、隐藏功能或切换到语义更差的实现。
- 允许的降级必须经过设计、显式配置、能力等价、可观测且可恢复；安全、账务和租户边界默认不允许隐式 Fail Open。
- Fallback 目标必须满足请求所需能力。Tools、Reasoning、Structured Output、Multimodal、Usage 等语义不得静默丢失。
- 多写有诊断价值的结构化调试日志，但不得记录 API Key、Authorization、完整 Prompt/Response、未脱敏 PII 或其他秘密。
- 新增或修改行为必须覆盖成功、失败、边界和并发路径；修 Bug 必须有能证明根因的回归测试。
- 不修改生成产物来代替修改源文件。构建产物策略按 v2 的 `BASE-001`/ADR-002 治理。
- 不覆盖、不删除、不暂存与当前任务无关的用户修改。

## 5. 完成定义

完成至少意味着：根因被处理、职责边界合理、相关测试通过、错误路径可追踪、文档与配置同步、changelog 已生成、Git 提交已创建。任一环节因客观原因无法完成，必须明确报告，不能悄悄省略。
