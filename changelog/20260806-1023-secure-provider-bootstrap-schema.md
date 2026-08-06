# 收敛安全 Provider Bootstrap Schema

- 日期：2026-08-06 10:23 CST
- 类型：L2
- 关联需求：Task 4
- 影响模块：`config/**`、`cmd/gateway`、`internal/server`、README 与 Bootstrap 配置文档

## 根因

旧示例把 DeepSeek、SiliconFlow、豆包等厂商统一声明为 `type: openai`，把“请求字段相似”误当成已经完成厂商协议适配；同时示例包含固定 Gateway Key，Provider Secret 也只能通过 `api_key` 值进入普通配置。该契约无法区分厂商 Endpoint、地域和协议版本，也无法诚实表达 Adapter 尚未实现、能力尚未验证的状态。

## 解决方案

- 为 Ark、DeepSeek、Qwen 增加独立 `kind` 和厂商专属 Endpoint Schema，严格校验受控 HTTPS Host、Path、region、protocol version，以及 Qwen Workspace ID 与 Host 的对应关系。
- Native Provider 只保存 `credential.env` 环境变量名称，不解析或保存秘密值；M0 阶段强制 `enabled: false` 与 `evidence.status: unverified`。
- Route 拒绝引用 disabled Provider；误启用未实现 Adapter 时在配置加载边界返回带字段路径和恢复动作的错误。
- 保留遗留 OpenAI/Claude 字段用于 Strangler Migration 回归，但禁止与 Native Schema 混用。
- 删除首批范围外默认 Provider/Route 和固定 Gateway Key；用 `.invalid` 保留域名、明显无效占位 Credential 与模型保持当前进程可复现启动且无真实上游权限。

## 行为与兼容性

现有合法 OpenAI/Claude 迁移配置保持兼容。新增 Native 声明不会构造运行时 Provider，且不能成为 Route Target。默认示例不再调用真实厂商；Ark、DeepSeek、Qwen 在对应 M3 Adapter 完成前都不构成支持声明。Qwen Bootstrap 示例模型固定为 `qwen3.7-flash`。

## 可观测性

启动组装会以结构化字段 `provider`、`provider_kind`、`evidence_status` 记录被跳过的 disabled Bootstrap 声明，不记录 Credential 值。没有可运行 Provider 时返回明确错误，不静默构造空网关。

## 验证

- `[通过]` `C:\Program Files\Go\bin\go.exe test ./config ./cmd/gateway ./internal/server`
- `[通过]` `C:\Program Files\Go\bin\go.exe test ./...`
- `[通过]` `C:\Program Files\Go\bin\go.exe vet ./...`
- `[通过]` `C:\Program Files\Go\bin\go.exe build -o NUL ./cmd/gateway`
- `[通过]` README Bootstrap 声明由 `TestREADMEDeclaresNativeProvidersAsBootstrapOnly` 检查。
- `[通过]` 已对工作树执行高熵 Credential assignment 与常见 Token 前缀降级扫描，无匹配；本机未安装 Gitleaks。
- `[未执行]` `go test -race ./config ./internal/server`：已显式重试 `CGO_ENABLED=1`，但本机不存在 `gcc`，Go Race 构建无法启动；Task 4 未引入并发状态，常规测试已通过。

## 风险与回滚

Native Schema 当前仅允许受控的北京 Ark/Qwen Endpoint 与 DeepSeek global stable/beta 形态，其他地域需要后续基于官方文档扩展，不能通过任意 Base URL 绕过。回滚本提交可恢复旧 Schema，但也会恢复固定示例 Key、模糊厂商声明和范围外路由，不建议单独回滚。无数据迁移。
