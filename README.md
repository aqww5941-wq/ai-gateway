# AI Gateway

> 正在按 v3 任务书演进的 LLM 网关：当前 M0 先建立可信构建、配置和安全基线。

## 解决什么问题

LLM 接入需要统一处理凭据、协议差异、路由、韧性、限流配额、流式响应和 Usage。项目目标是为业务方提供统一入口，但当前代码仍处于迁移期：遗留 OpenAI/Claude 转发路径用于回归，Ark、DeepSeek、Qwen Native Adapter 将在 M3 完成，现阶段不得视为已支持。

## 核心特性

| 模块 | 说明 |
| --- | --- |
| **Provider Bootstrap** | Ark / DeepSeek / Qwen 使用独立 Kind、Endpoint 和 Secret reference；当前固定为 disabled/unverified，Native Adapter 尚未实现 |
| **路由策略** | round_robin / semantic（按 prompt 复杂度分级）/ latency（p99 延迟反比加权） |
| **韧性** | 熔断器 + 指数退避重试 + singleflight 合并 + 延迟感知路由 |
| **企业并发控制** | max-in-flight 限流 + 请求队列 + 共享 HTTP 连接池 + 2MB body 限制 |
| **缓存** | 精确匹配 + 语义匹配，内存 / Redis 双后端，流式也能命中 |
| **鉴权与 RBAC** | API Key + admin/user 角色 + 模型级别访问控制，SHA-256 哈希存储 + 常数时间比较 |
| **配额管理** | 每日/每月 token 配额，响应头实时返回剩余额度 |
| **敏感信息过滤** | 自动检测并 mask/block 手机号、身份证、邮箱、银行卡、API Key |
| **审计日志** | 每次请求记录调用方、模型、token 消耗、延迟，30 天自动清理 |
| **管理后台** | React SPA 内嵌二进制，单文件部署，Key 管理 / 配额 / 熔断器 / 缓存 / 审计一站式 |
| **可观测** | Prometheus 指标 + OpenTelemetry 链路追踪 + 结构化 slog 日志 + 请求成本估算 |
| **受控热重载** | Provider/Route 以单调 revision 原子发布；Server/Auth/Quota/Cache/Filter 等启动资源变更明确要求重启 |
| **零框架** | 仅依赖 yaml、sqlite、redis、OTel SDK，无 Web 框架 |

## 快速开始

### 1. 编译

```bash
git clone https://github.com/your-org/ai-gateway.git
cd ai-gateway

# 跨平台完整构建：npm ci、前端产物生成、Go 二进制编译
go run ./cmd/build
```

Windows 输出 `bin/gateway.exe`，Linux/macOS 输出 `bin/gateway`。也可以运行
`make build`，它会委托给同一个 Go 构建入口。仅构建前端或后端时，分别使用
`go run ./cmd/build -target frontend` 和 `go run ./cmd/build -target backend`。
前端只生成到 `internal/static/dist/`，该目录也是 Go 的唯一嵌入输入。

### 2. 检查安全 Bootstrap 示例

`config/gateway.yaml` 不包含可用的 Gateway Key 或 Provider Credential。三家 Provider 只保存
`ARK_API_KEY`、`DEEPSEEK_API_KEY`、`DASHSCOPE_API_KEY` 这些环境变量名称，不读取或复制其值；
默认路由指向 `.invalid` 保留域名，因此没有真实上游调用权限。

### 3. 启动

```bash
# Linux / macOS
./bin/gateway                          # 默认读取 config/gateway.yaml
./bin/gateway -config /path/to.yaml    # 指定配置文件

# Windows PowerShell
.\bin\gateway.exe
.\bin\gateway.exe -config C:\path\to\gateway.yaml
```

网关监听 `:8081`，管理后台 `http://localhost:8081/admin/dashboard/`。安全示例可以启动进程，
但不能成功调用真实模型；不要通过把 Native Provider 改成通用 `type: openai` 来绕过适配器边界。

### 4. 当前调用边界

Task 4 不实现真实厂商调用。把任一 Native Provider 的 `enabled` 改为 `true`，配置加载会给出
“adapter ... is not implemented; set enabled to false”的可操作错误。真实 API Smoke 只会在对应
Native Adapter 完成后以显式 opt-in 方式运行；Qwen 的预定 Smoke 模型为 `qwen3.7-flash`。

## 架构

```
Client
  │  POST /v1/chat/completions
  ▼
┌─────────────────────────────────────────────────┐
│  Concurrency Limiter (max-in-flight + queue)     │
├─────────────────────────────────────────────────┤
│  Auth (API Key → Identity + RBAC)               │
├─────────────────────────────────────────────────┤
│  Quota Check (daily/monthly token limit)         │
├─────────────────────────────────────────────────┤
│  Rate Limiter (token bucket, per-key + per-model)│
├─────────────────────────────────────────────────┤
│  Filter (sensitive content mask/block)           │
├─────────────────────────────────────────────────┤
│  Cache (exact + semantic, memory/redis)          │
├─────────────────────────────────────────────────┤
│  Router (round_robin / semantic / latency)       │
├─────────────────────────────────────────────────┤
│  Singleflight (coalesce concurrent identical)    │
├─────────────────────────────────────────────────┤
│  Retry (exponential backoff + jitter, 429-aware) │
├─────────────────────────────────────────────────┤
│  Circuit Breaker (5 failures → open, 10s cooldown)│
├─────────────────────────────────────────────────┤
│  Provider Call (shared HTTP transport pool)      │
└─────────────────────────────────────────────────┘
  │
  ▼
Upstream LLM APIs
```

## 配置

`config/gateway.yaml` 是单一配置源，主要区块：

配置加载会拒绝未知字段、缺失环境变量、非法范围和悬空路由引用；默认值与完整约束见
[`docs/configuration.md`](docs/configuration.md)。

```yaml
server:
  port: 8081
  db_path: data/gateway.db          # SQLite 路径
  max_concurrency: 500              # 最大并发请求数
  queue_size: 200                   # 等待队列长度
  queue_timeout: 10s                # 队列超时
  transport:
    max_conns_per_host: 100         # 每上游最大连接数
    max_idle_conns_per_host: 50
    max_idle_conns: 200

auth:
  enabled: false                    # 仓库不分发固定 Gateway Key
  keys: []

providers:
  - name: legacy-invalid-example   # 仅保证当前遗留进程可启动
    type: openai
    api_key: invalid-example-provider-key
    base_url: https://example.invalid/v1
    models: [invalid-example-model]

  - name: ark-bootstrap
    kind: ark
    enabled: false
    credential: { env: ARK_API_KEY }
    evidence: { status: unverified }
    ark:
      base_url: https://ark.cn-beijing.volces.com/api/v3
      region: cn-beijing
      protocol_version: responses-v1
      endpoint_id: invalid-example-ark-endpoint-id
    models: [invalid-example-ark-model]

  - name: deepseek-bootstrap
    kind: deepseek
    enabled: false
    credential: { env: DEEPSEEK_API_KEY }
    evidence: { status: unverified }
    deepseek:
      base_url: https://api.deepseek.com
      region: global
      protocol_version: chat-completions-v1
      endpoint: stable
    models: [deepseek-v4-flash, deepseek-v4-pro]

  - name: qwen-bootstrap
    kind: qwen
    enabled: false
    credential: { env: DASHSCOPE_API_KEY }
    evidence: { status: unverified }
    qwen:
      base_url: https://invalid-example-workspace-id.cn-beijing.maas.aliyuncs.com/compatible-mode/v1
      region: cn-beijing
      protocol_version: chat-completions-v1
      workspace_id: invalid-example-workspace-id
    models: [qwen3.7-flash]

routes:
  - name: invalid-bootstrap-route
    match: { model: invalid-example-model }
    strategy: round_robin
    targets: [{ provider: legacy-invalid-example, model: invalid-example-model }]

cache:
  enabled: true
  backend: memory                   # memory | redis
  ttl: 1h
  strategy: exact                   # exact | semantic
  max_size: 1000

filter:                             # 敏感信息过滤
  enabled: true
  mode: mask                        # mask | block
  rules: [phone_cn, id_card_cn, email, credit_card, api_key]

quota:
  enabled: true                     # 每日/每月 token 配额

rate_limit:
  enabled: false                    # 令牌桶限流
  per_key: 60
  per_model: 100
```

## 管理后台

启动后访问 `http://localhost:8081/admin/dashboard/`。默认安全示例没有预置 admin Key；只有在显式配置鉴权凭据后才能以 admin 身份调用管理 API。

| 页面 | 功能 |
| --- | --- |
| **Overview** | 请求量、缓存命中率、错误率、流式占比概览 |
| **Routes** | 查看当前所有路由规则 |
| **Cache** | 缓存命中率、容量、逐条查看缓存内容 |
| **Breakers** | 各厂商熔断器状态（closed/open/half-open） |
| **Providers** | 上游厂商延迟、成功率、连接池状态 |
| **Keys** | 创建/编辑/禁用 API Key，查看每 Key 配额使用 |
| **Logs** | 审计日志查询，按 Key 筛选，分页浏览 |
| **Filter** | 敏感信息过滤规则管理 |

## Admin API

所有管理接口需 admin 角色，详见 `internal/server/admin_api.go`：

```
GET    /admin/health
GET    /admin/api/v1/overview
GET    /admin/api/v1/routes
GET    /admin/api/v1/cache
GET    /admin/api/v1/cache/entries/{hash}
GET    /admin/api/v1/breakers
GET    /admin/api/v1/providers
GET    /admin/api/v1/latency
GET    /admin/api/v1/quotas
GET    /admin/api/v1/keys
POST   /admin/api/v1/keys           # 创建 Key，返回明文 token（仅此一次）
PUT    /admin/api/v1/keys/{id}
DELETE /admin/api/v1/keys/{id}
GET    /admin/api/v1/audit-logs?key=&limit=&offset=
GET    /admin/api/v1/filter
GET    /metrics                     # Prometheus 指标
```

## 响应头

每次 API 请求返回以下限流相关头：

| Header | 含义 |
| --- | --- |
| `X-RateLimit-Limit-Requests` | 每日 token 上限 |
| `X-RateLimit-Remaining-Requests` | 当日剩余 token |
| `X-RateLimit-Reset-Requests` | 重置时间 Unix 时间戳（UTC 次日 0 点） |

## 项目结构

```
ai-gateway/
├── cmd/
│   ├── build/main.go               # 跨平台统一构建入口
│   └── gateway/main.go             # 网关入口
├── config/
│   ├── config.go                   # YAML 加载
│   ├── reloader.go                 # fsnotify 热重载
│   └── gateway.yaml                # 配置样例
├── internal/
│   ├── breaker/                    # 熔断器（锁无关 atomic）
│   ├── cache/                      # 精确+语义缓存，内存+Redis，singleflight
│   ├── filter/                     # 敏感信息检测与脱敏
│   ├── limiter/                    # 令牌桶限流
│   ├── metrics/                    # Prometheus 指标注册
│   ├── middleware/                 # Auth / Quota / RateLimit / Cache / Concurrency
│   ├── observer/                   # 请求级观测 + 成本估算
│   ├── provider/                   # OpenAI 协议适配器 + 共享 Transport
│   ├── retry/                      # 指数退避重试（全抖动，429 感知）
│   ├── router/                     # round_robin / semantic / latency / fallback
│   ├── server/                     # HTTP Server + Admin API + 请求处理
│   ├── static/
│   │   └── dist/                   # 唯一前端构建产物与 //go:embed 输入
│   ├── store/                      # SQLite — 鉴权 / 配额 / 审计
│   └── tracing/                    # OpenTelemetry 封装
├── web/                            # React 管理后台前端（web/dist 已停用并忽略）
│   ├── src/
│   │   ├── pages/                  # 8 个页面组件
│   │   ├── components/            # 通用 UI 组件
│   │   ├── api.ts                 # API 请求层
│   │   └── App.tsx                # 主布局 + 路由
│   └── package.json
├── go.mod
└── go.sum
```

## 测试

```bash
# 全部测试
go test ./...

# 基准测试
go test -bench=. -benchmem ./...
```

## Docker 部署

```dockerfile
FROM node:22-alpine AS web
WORKDIR /app/web
COPY web/ ./
RUN npm ci && npm run build

FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /app/internal/static/dist ./internal/static/dist
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -o gateway ./cmd/gateway

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/gateway /usr/local/bin/gateway
COPY --from=builder /app/config/gateway.yaml /etc/gateway/config.yaml
EXPOSE 8081
CMD ["gateway", "-config", "/etc/gateway/config.yaml"]
```

## 许可

MIT
