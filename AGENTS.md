# AI Gateway Agent 工作入口

本文件是所有 Agent 进入仓库后的唯一入口。项目事实、工程约束和场景规则位于 `.rules/`；架构目标位于 `docs/AI_Gateway_v3_企业级重构设计文档.md`；逐项实施顺序位于 `docs/AI_Gateway_v3_项目实施任务书.md`。

## 1. 当前任务与目标架构

- 项目目标是构建可验证的多厂商 LLM 网关，而不是简单的 OpenAI Base URL 代理。
- 当前代码仍是 Go `net/http` 数据面、React 管理端和旧 `provider.ChatRequest` 模型；它是迁移起点，不是目标架构。
- v3 目标采用 Gin 构建数据面与控制面两个独立入口，核心应用层只依赖 `context.Context`、DTO 和 Ports，不依赖 `gin.Context`。
- 首批真实上游固定为火山引擎方舟、DeepSeek、阿里云百炼 Qwen。客户端入口可以兼容 OpenAI Chat Completions/Responses，但不得把“协议兼容”写成“已接入 OpenAI 厂商”。
- 每个厂商必须有独立 Adapter、能力矩阵、Golden/Conformance Fixture 和可选真实 API Smoke Test；禁止用一个通用 OpenAI-compatible 转发器冒充完整适配。
- 新协议能力进入 Canonical IR、Capability Planner 和 Adapter 边界，不得继续把厂商字段堆入旧 `provider.ChatRequest`。
- v3 实施必须按任务书 Task 1～Task 62 顺序推进。同一时间只允许一个 Task 进入实现；当前 Task 验收、changelog 和原子提交完成后才能开始下一项。

## 2. M0 是什么

`M` 表示 Milestone（里程碑），`M0` 是第 0 个里程碑：先建立可信工程基线，再进行架构重构。它不是业务版本号，也不是“先做一个简陋 MVP”。

M0 必须固定当前行为并解决构建、配置、安全示例、测试、CI 和生成产物治理。如果基线不可复现，后续 Gin、Canonical IR 或 Adapter 的失败就无法判断是新回归还是旧问题。实施顺序遵循 v3 文档 M0 至 M6；不得绕过前一阶段 Exit Gate 堆叠后续功能。

## 3. 每次任务的必读与强制工作流

每次开始必须：

1. 运行 `git status --short` 与 `git branch --show-current`，识别并保护用户已有修改。
2. 完整阅读 `.rules/00-project.md`。
3. 修改文件时完整阅读 `.rules/01-engineering-principles.md`、`.rules/02-delivery-workflow.md` 和所有命中的场景规则。
4. 修改运行时代码时额外阅读 `.rules/03-observability.md`。
5. v3 实现任务读取 `docs/AI_Gateway_v3_项目实施任务书.md`，确认唯一 Ready Task、依赖、范围、验收和禁止项；不得跳 Task 或把多个 Task 合成一次大改。
6. 修改前写清：现象、触发条件、根因、影响范围、设计方案和验证方式。禁止未定位根因就止血式修补。
7. 实现完整方案，同步测试、诊断日志、指标、Trace、配置、文档或迁移。
8. 按风险验证；未执行或未通过的检查必须如实记录。
9. 代码、配置契约、数据库、API、构建、测试行为或治理规则变化时，在 `changelog/` 新建记录。
10. 只暂存本 Task 文件并创建一个原子本地提交。默认不 push，不覆盖用户修改。

纯阅读、分析或代码审查不生成 changelog，也不提交。

## 4. 场景规则索引

| 场景或路径 | 必读规则 |
| --- | --- |
| `cmd/**`、`internal/**`、`config/**` 的 Go 代码 | `.rules/scenarios/go-backend.md` |
| Gin、HTTP、Middleware、SSE、数据面/控制面 | `.rules/scenarios/http-gin.md` |
| Provider、Canonical IR、厂商 Codec、SSE 协议 | `.rules/scenarios/provider-protocol.md` |
| Router、Retry、Breaker、Fallback、Capability Planner | `.rules/scenarios/routing-resilience.md` |
| 配置、默认值、校验、热重载、Runtime Snapshot | `.rules/scenarios/config-runtime.md` |
| Store、鉴权、Key、Credential、Quota、Ledger、租户 | `.rules/scenarios/storage-security.md` |
| `web/**` | `.rules/scenarios/web-frontend.md` |
| API、Schema、公共接口、兼容性、架构决策、文档 | `.rules/scenarios/docs-adr.md` |

同一变更命中多个场景时，必须读取全部相关规则。

## 5. 不可绕过的不变量

- 不吞错、不伪造成功、不把未验证能力写成已支持。
- 不通过删除 Tools、Reasoning、Structured Output、Multimodal、Usage 或厂商状态字段实现“兼容”。无法保真时，Planner 必须拒绝或选择已证明兼容的目标。
- 安全、额度、账务和租户边界默认 Fail Closed；任何降级必须显式配置、可观测、可恢复且有风险上限。
- Retry/Fallback 只能发生在客户端响应 Commit 之前，并满足能力、策略和预算约束。
- 不记录 API Key、Authorization、完整 Prompt/Response、Tool 参数原文、未脱敏 PII 或可重放的完整 SSE。
- 新行为覆盖成功、失败、边界和相关并发路径；Bug 必须有能证明根因的回归测试。
- 不修改生成产物代替源文件，不覆盖、不删除、不暂存与任务无关的用户修改。

## 6. 完成定义

完成至少意味着：根因被消除、职责边界符合 v3、相关测试通过、错误路径可追踪、契约和文档同步、changelog 已创建、原子提交已完成。任何一项无法完成都必须明确报告，不能静默省略。
