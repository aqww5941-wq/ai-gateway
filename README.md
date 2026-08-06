# AI Gateway

> 多 LLM 厂商统一代理网关 — 一个 OpenAI 兼容入口，接管路由、韧性、缓存、鉴权、配额、过滤、可观测。

## 解决什么问题

接入多家 LLM 厂商（DeepSeek / SiliconFlow / 豆包等），业务代码反复处理 key 轮询、失败重试、限流配额、模型路由、流式兼容、token 计费。AI Gateway 把这些横切关注点抽离为基础设施，业务方只需调一个 `POST /v1/chat/completions`。

## 核心特性

| 模块 | 说明 |
| --- | --- |
| **多厂商适配** | OpenAI 协议适配器，覆盖 DeepSeek / SiliconFlow / 豆包等所有 OpenAI 兼容端点 |
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
| **热重载** | fsnotify 监听配置文件，改完即生效，不停机 |
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

### 2. 配置环境变量

```bash
export DEEPSEEK_API_KEY=sk-...
export SILICONFLOW_API_KEY=sk-...
export DOUBAO_API_KEY=...
```

### 3. 启动

```bash
# Linux / macOS
./bin/gateway                          # 默认读取 config/gateway.yaml
./bin/gateway -config /path/to.yaml    # 指定配置文件

# Windows PowerShell
.\bin\gateway.exe
.\bin\gateway.exe -config C:\path\to\gateway.yaml
```

网关监听 `:8081`，管理后台 `http://localhost:8081/admin/dashboard/`。

### 4. 第一个请求

```bash
# 非流式
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Authorization: Bearer sk-test-123" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"你好"}]}'

# 流式
curl -N -X POST http://localhost:8081/v1/chat/completions \
  -H "Authorization: Bearer sk-test-123" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-v4-flash","stream":true,"messages":[{"role":"user","content":"讲个笑话"}]}'

# 语义路由 — simple → mini, complex → pro
curl -X POST http://localhost:8081/v1/chat/completions \
  -H "Authorization: Bearer sk-test-123" \
  -H "Content-Type: application/json" \
  -d '{"model":"doubao","messages":[{"role":"user","content":"用 Go 实现红黑树"}]}'
```

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

auth:                               # 鉴权 + RBAC
  enabled: true
  keys:                             # 种子 Key（仅首次启动导入）
    - token: "sk-test-123"
      name: "admin"
      role: admin                   # admin | user
      daily_token_limit: 1000000

providers:                          # 上游厂商
  - name: deepseek
    type: openai
    api_key: ${DEEPSEEK_API_KEY}    # 环境变量
    base_url: https://api.deepseek.com/v1
    models: [deepseek-v4-flash, deepseek-v4-pro]

routes:                             # 路由规则
  - name: cheap
    match: { model: deepseek-v4-flash }
    strategy: round_robin
    targets: [{ provider: deepseek, model: deepseek-v4-flash }]

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

启动后访问 `http://localhost:8081/admin/dashboard/`，使用 admin 角色的 API Key 登录。

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
