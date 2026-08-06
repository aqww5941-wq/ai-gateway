# AI Gateway

一个正在按 v3 任务书演进的 Go LLM Gateway。当前已通过 **M0 可信基线** Exit Gate：已有
`net/http` 数据入口、React 管理端、配置、路由、缓存、重试、熔断、鉴权、Quota、审计和
可观测代码。M0 只证明仓库具备安全开始架构迁移的工程基线，本仓库仍不声明“生产级”“企业级”或“全厂商兼容”。

## 事实等级

- **Implemented**：代码存在，相关自动测试通过。
- **Experimental**：可以运行，但契约、错误语义或验证仍不完整。
- **Planned**：已经进入 v3 设计和任务书，尚未实现。
- **Unverified**：没有对应真实厂商、模型、地域和 Endpoint 的 Smoke 证据。

`OpenAI-compatible` 在本文只表示客户端或上游请求形状的兼容子集，不表示已经适配 OpenAI，
也不表示任意兼容厂商的 Tools、Reasoning、Structured Output、Multimodal、Streaming 或 Usage
语义能够保真。

## 当前能力矩阵

| 能力 | 状态 | 当前证据与边界 |
| --- | --- | --- |
| 跨平台完整构建 | Implemented | `go run ./cmd/build` 统一执行锁定依赖的前端构建和 Go 构建；Windows 本地与 Linux CI 已验证 |
| Go、前端与 Secret 质量门禁 | Implemented | GitHub Actions 执行 Format、Test、Race、Vet、Staticcheck、Actionlint、Frontend quality、Build 和 Gitleaks |
| 严格 Bootstrap 配置 | Implemented | 未知字段、缺失环境变量、非法范围、重复标识和悬空路由会被拒绝 |
| Provider/Route 原子热重载 | Implemented | 每个请求固定使用一个单调 revision；其他启动资源变更明确要求重启 |
| `POST /v1/chat/completions` Ingress | Experimental | 仅文本 Chat 子集，支持 Unary 与 SSE；离线端到端 Fixture 已通过，没有版本化 OpenAPI |
| 遗留 `openai` Egress | Experimental | `base_url + /chat/completions` 的文本 Unary/SSE 转发；仅有离线合约证据，不代表任何真实厂商已验证 |
| 遗留 `claude` Egress | Experimental / Unverified | 仅 Messages Unary 文本转换；不支持 Streaming，未做真实 Anthropic 验证 |
| Ark Native Adapter | Planned / Unverified | 当前只有独立 Bootstrap Schema，强制 `enabled: false`；M3 Task 28～30 实现与验证 |
| DeepSeek Native Adapter | Planned / Unverified | 当前只有独立 Bootstrap Schema，强制 `enabled: false`；M3 Task 31～32 实现与验证 |
| Qwen Native Adapter | Planned / Unverified | 当前只有独立 Bootstrap Schema，强制 `enabled: false`；M3 Task 33～34 实现与验证 |
| SQLite Key、Quota、Audit | Experimental | 存储与并发行为已有保护测试，但原子预算、月额度、Fail Closed 和幂等结算尚未完成 |
| React Admin 与 Admin API | Experimental | SPA 和现有管理接口可运行；尚无 OpenAPI、统一错误 Envelope、OIDC 或独立控制面 |
| Gin 双平面、Canonical IR、Responses Ingress | Planned | 分别由 M1、M2 实施；当前代码仍是单端口 `net/http` 和旧 `provider.ChatRequest` |

最新远端质量证据：[`Quality` Workflow on master](https://github.com/aqww5941-wq/ai-gateway/actions/runs/31080132804)。
真实 Provider 能力只有在记录 `provider + endpoint + region + model + protocol version + adapter revision`
的 opt-in Smoke 后，才会从 Unverified 升级。

## 快速开始

### 1. 环境要求

| 场景 | 必需工具 |
| --- | --- |
| 完整构建 | Go 1.26.4、Node.js 22.14.0、npm 10.9.x、Git |
| 只构建现有 Go 网关 | Go 1.26.4；仓库已跟踪管理端嵌入产物，不要求 Node.js |
| 本地完整质量门禁 | 上述工具；Windows Race 可按质量文档使用 WSL |

版本事实源分别是 `go.mod`、`web/.node-version`、`web/package.json` 和 lockfile。

### 2. 获取并构建

```bash
git clone https://github.com/aqww5941-wq/ai-gateway.git
cd ai-gateway

# 完整构建：npm ci -> React 产物 -> Go 二进制
go run ./cmd/build

# 不重建前端，只使用已跟踪的嵌入产物构建网关
go run ./cmd/build -target backend
```

完整构建输出：Windows 为 `bin/gateway.exe`，Linux/macOS 为 `bin/gateway`。构建产物契约和
分阶段命令见 [`docs/build.md`](docs/build.md)。

### 3. 理解安全示例

默认 [`config/gateway.yaml`](config/gateway.yaml) 可以启动，但**不能成功调用真实模型**：

- 不分发固定 Gateway Key，`auth.enabled` 为 `false`。
- 唯一启用的遗留 Provider 使用 `example.invalid` 和无效占位 Credential。
- Ark、DeepSeek、Qwen 仅为禁用的 Schema 声明，`evidence.status` 为 `unverified`。
- Native 声明只保存 `ARK_API_KEY`、`DEEPSEEK_API_KEY`、`DASHSCOPE_API_KEY` 这些环境变量名，
  不读取或复制 Secret 值。
- `rate_limit` 与 `quota` 默认关闭；Cache 和敏感信息 Mask 默认开启。

把 Native Provider 的 `enabled` 改成 `true` 会得到可操作的配置错误；不要把它改成遗留
`type: openai` 来绕过 Adapter 边界。

下面只展示可公开提交的 Bootstrap 事实；`env` 的值是环境变量名称，不是 Credential：

```yaml
- kind: ark
  enabled: false
  credential: {env: ARK_API_KEY}
  evidence: {status: unverified}

- kind: deepseek
  enabled: false
  credential: {env: DEEPSEEK_API_KEY}
  evidence: {status: unverified}

- kind: qwen
  enabled: false
  credential: {env: DASHSCOPE_API_KEY}
  evidence: {status: unverified}
  models: [qwen3.7-flash]
```

三家 Native Adapter 尚未实现；完整且可加载的字段以 `config/gateway.yaml` 为准。

### 4. 启动

Linux/macOS：

```bash
./bin/gateway -config config/gateway.yaml

# 可选 Unix 前台包装器：只构建后端，然后 exec 网关
./start.sh
./start.sh /absolute/path/to/gateway.yaml
```

Windows PowerShell：

```powershell
.\bin\gateway.exe -config .\config\gateway.yaml
.\bin\gateway.exe -config C:\path\to\gateway.yaml
```

进程默认监听 `:8081`，管理 SPA 位于 <http://localhost:8081/admin/dashboard/>。默认安全配置
没有 admin 身份，因此管理 API（包括 `GET /admin/health`）会返回 `403`；这不是启动失败。
`/admin/health` 当前只返回静态状态，并不是 readiness。项目目前没有独立 liveness/readiness
Endpoint，Task 15 才会建立双 Server 健康边界。

停止前台进程请使用 `Ctrl+C`，让网关执行 Graceful Shutdown。仓库不再提供“按端口强杀进程”
的停止脚本；服务化运行应交给 systemd、容器运行时或其他明确持有进程 PID 的 Supervisor。

## 当前 HTTP 边界

数据入口只有：

```text
POST /v1/chat/completions
```

它接收的旧请求模型只有 `model`、字符串 `messages[]`、`temperature`、`max_tokens` 和 `stream`。
当前没有 `/v1/responses`，也不能表达 Tools、Tool Result、Reasoning、Structured Output、
Multimodal、Citation 或 Provider-managed State。

同一 `:8081` 端口还承载：

| 路径 | 当前边界 |
| --- | --- |
| `/admin/dashboard/**` | 公开提供静态 SPA；管理 API 仍需要 admin 身份 |
| `/admin/**` | Auth 开启时需要有效 Gateway Key，随后必须通过 `role=admin` |
| `/metrics` | 当前无鉴权的 Prometheus Endpoint |

Admin v1 包含 Overview、Routes、Cache、Breakers、Providers、Latency、Quotas、Keys、Audit Logs
和 Filter 查询/管理接口，实际路由以 [`internal/server/admin.go`](internal/server/admin.go) 为准。

## 当前实现结构

```text
Client
  -> single net/http Server (:8081)
  -> Auth / Quota / RateLimit / Concurrency / Filter / Cache
  -> Router / Singleflight / Retry / Breaker
  -> legacy Provider
  -> upstream

React Admin -> same Server -> /admin/api/v1/**
```

当前路由实现包含 `round_robin`、`weighted`、`fallback`、`semantic` 和 `latency`；缓存实现包含
Memory/Redis 与 Exact/Semantic；已有 Prometheus、OpenTelemetry 和结构化 `slog`。这些模块是
迁移资产，不代表已经满足 v3 的 Capability、租户、账本、流状态机或多实例不变量。

目标架构、边界与取舍见
[`docs/AI_Gateway_v3_企业级重构设计文档.md`](docs/AI_Gateway_v3_企业级重构设计文档.md)，
逐 Task 依赖和验收见
[`docs/AI_Gateway_v3_项目实施任务书.md`](docs/AI_Gateway_v3_项目实施任务书.md)。

## Secret 与配置

- 普通配置字段支持 `$NAME` / `${NAME}` 环境变量展开；缺失变量会在加载阶段统一报错。
- Native Provider 的 `credential.env` 必须填写环境变量**名称**，不能填写明文值或 `${...}`。
- 遗留 Provider 的 `api_key` 和 Gateway `auth.keys[].token` 若用于本地迁移测试，应引用环境变量，
  不得把值提交到 YAML、日志、Fixture 或测试报告。
- 启动和 Reload 使用同一严格 Loader；只有 `providers` 与 `routes` 可动态发布，其余区块变化
  返回 restart-required。

完整字段、默认值、校验和 Reload 契约见
[`docs/configuration.md`](docs/configuration.md)。

## 已知限制

- M0 Exit Gate 只证明可信迁移基线；M1～M6 尚未完成，当前不能作为生产就绪结论。
- 数据面、管理端、Metrics 共用端口；没有独立控制面、liveness 或 readiness。
- Chat/SSE 仍使用文本最小模型；上游 SSE 解析或扫描失败目前可能被表现为 Channel 正常关闭和
  `[DONE]`，不能据此证明流完整成功。
- Fallback 对部分非重试型上游错误仍可能继续，尚未进行 Capability 等价校验。
- Quota 使用请求前检查与请求后记账，不能防止并发超额；月额度尚未参与拒绝，Store 故障路径
  仍存在 Fail Open / Best Effort 行为。
- Auth 撤销受一分钟缓存刷新影响；Admin SPA 当前把 Token 存在浏览器 `localStorage`。
- Redis 连接失败会回退本机内存缓存，不适合作为多实例一致性保证。
- Admin API 没有 OpenAPI、统一错误 Envelope、分页/幂等完整契约；`/metrics` 当前公开。
- 仓库没有 Dockerfile 或 Compose。容器、多实例、PostgreSQL/Redis Smoke 属于 M6 Task 58，
  在完成前不提供未经验证的部署配方。

详细迁移回归事实见 [`docs/baseline/`](docs/baseline/)。

## 质量验证

常用入口：

```bash
go run ./cmd/quality
go test -count=1 ./...
go vet ./...

cd web
npm ci
npm run quality
```

Race、Staticcheck、Actionlint、Gitleaks、负向 Probe 和生成产物检查见
[`docs/quality-gates.md`](docs/quality-gates.md)。Fixture/Mock 只证明离线行为，不会把真实厂商状态
标记为 Verified。

## 项目结构

```text
cmd/build/              跨平台统一构建入口
cmd/gateway/            当前网关进程入口
cmd/quality/            跨平台 Go Format Checker
config/                 Bootstrap Schema、严格加载与 Reload
internal/provider/      遗留 Provider 与离线合约 Fixture
internal/server/        当前 net/http 数据/管理入口
internal/store/         SQLite Key、Quota、Audit
internal/static/dist/   唯一跟踪的管理端构建产物
web/                    React 管理端源码
docs/                   v3 设计、任务书、契约、基线与质量证据
changelog/              原子变更记录
```

## 许可

[MIT](LICENSE)
