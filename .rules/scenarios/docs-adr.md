# 文档、ADR、API 与兼容性规则

## 1. 事实等级

- `Implemented`：代码存在且相关自动测试通过。
- `Verified`：在 Implemented 基础上通过指定真实厂商/环境验证，并记录证据。
- `Planned`：设计已明确但未完成。
- `Experimental`：可运行但契约或验证不完整。
- `Unsupported/Unverified`：明确不支持或尚无真实证据。

README、管理端、发布说明和简历不得混用这些等级。“企业级”“生产级”“全厂商兼容”和性能数字必须有对应 Exit Gate 与可复现报告。

## 2. 架构与 ADR

- v3 设计是当前迁移基线；重大变化先更新设计或创建 ADR，并写 Context、Decision、Alternatives、Consequences、Migration 和 Rollback。
- Gin 边界、Canonical IR、Capability 模型、Provider Adapter、Commit Barrier、Snapshot、Ledger 等核心决策不得只存在代码注释。
- 文档描述现状时使用当前代码证据，描述目标时标注 milestone；禁止把目标架构写成已实现。

## 3. 厂商与时效性

- 协议行为引用厂商官方一手文档，记录访问/验证日期；不以博客或 SDK 猜测 API 契约。
- 能力表写明 provider、model、region、endpoint、protocol version、adapter revision 和 evidence status。
- 客户端协议兼容与真实上游厂商适配分开表述。例如“支持 OpenAI Responses ingress”不能写成“已接入 OpenAI”。
- 官方文档更新时先做差异分析和 Fixture 回归，再更新能力状态。

## 4. API 与 Schema

- 外部 API 使用版本化 OpenAPI/JSON Schema，定义成功与错误 Envelope、分页、幂等和兼容规则。
- 增加可选字段通常向后兼容；删除/重命名、收紧校验、改变默认值或错误语义必须有迁移窗口。
- Provider 原始字段不直接泄露为稳定公共契约；通过 Canonical 或显式 vendor extension 命名空间暴露。

## 5. 变更记录

Changelog 解释根因、用户影响、迁移和实际验证，不逐行复述 diff。引用本地文档使用可稳定定位的仓库路径；引用在线协议使用官方直达链接。
