# AI Gateway

> 一个面向多 LLM 厂商的轻量、高韧性、生产级 API 网关。Go 1.26 编写,零业务侧依赖,部署即用。

## 它解决什么问题

当团队同时接入 DeepSeek / SiliconFlow / 豆包 / Claude 等多个 LLM 厂商时,通常会反复踩同一批坑:多 key 轮询、失败切换、限流、配额、缓存、可观测、模型路由、streaming 兼容、token 计费、prompt 缓存。AI Gateway 把这些横切关注点从业务代码里抽出来,只暴露一个 OpenAI 兼容的 `POST /v1/chat/completions` 入口。

## 核心特性

| 模块 | 说明 |
| --- | --- |
| **多厂商适配** | 内置 OpenAI 协议适配器(覆盖 DeepSeek / SiliconFlow / 豆包等所有 OpenAI 兼容端点)和 Anthropic Claude 适配器 |
| **路由策略** | `round_robin` / `weighted` / `latency`(按实测延迟反比加权)/ `semantic`(按 prompt 复杂度分级)/ `fallback`(链式故障转移) |
| **韧性** | 每厂商独立熔断器(5 次失败 / 10s 冷却 / 2 次半开探测)+ 指数退避重试(全抖动,识别 429/5xx,可读 `Retry-After`)+ 单飞合并(并发同请求合并到一次上游调用) |
| **缓存** | 精确匹配 + 语义匹配两档,内存 / Redis 双后端,SSE 流也能命中 |
| **限流** | 令牌桶,按 API key 与按模型两个维度,可单独开关 |
| **鉴权** | 常量时间比较的 API key 校验,前缀哈希 O(1) 查表,防计时侧信道 |
| **可观测** | Prometheus 指标 + OpenTelemetry 链路追踪(OTLP/HTTP 或 stdout 导出器)+ 结构化 slog 日志 + 单次请求成本估算 |
| **Streaming** | SSE 完整支持,客户端写与上游读两路扇出,无写竞争,客户端断连 ctx 即取消 |
| **热重载** | 监听 `gateway.yaml`,改完保存即生效,不停机 |
| **零业务侧依赖** | 除 yaml、redis、yaml.v3 与 OpenTelemetry 之外不引入任何业务框架 |

## 快速开始

### 编译

```bash
git clone <your-repo>
cd ai-gateway
go build -o bin/gateway ./cmd/gateway
```

### 配置

复制并按需修改 `config/gateway.yaml`,然后通过环境变量注入各厂商的 API key:

```bash
export DEEPSEEK_API_KEY=sk-...
export SILICONFLOW_API_KEY=sk-...
export DOUBAO_API_KEY=...
./bin/gateway -config config/gateway.yaml
```

监听 `:8080`。改 `config/gateway.yaml` 后无需重启,网关会自动重载。

### 第一个请求

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "deepseek-v4-flash",
    "messages": [{"role": "user", "content": "你好"}]
  }'
```

`model` 字段是**逻辑路由名**,由 `gateway.yaml` 的 `routes[].match.model` 决定命中哪条规则,实际打到哪个上游 provider/model 取决于策略选择。客户端无需关心后端细节。

## 配置

`config/gateway.yaml` 是单一事实源,字段含义如下:

```yaml
server:                 # HTTP 监听
  port: 8080
  read_timeout: 30s
  write_timeout: 120s

auth:                   # 客户端鉴权(可选)
  enabled: false
  keys: []              # 在此列出可调用的 API key

providers:              # 上游厂商清单
  - name: deepseek      # 网关内唯一标识
    type: openai        # openai | claude
    api_key: ${...}     # 支持 ${ENV} 展开
    base_url: https://api.deepseek.com/v1
    models: [deepseek-v4-flash, deepseek-v4-pro]
    timeout: 60s

routes:                 # 客户端 model → 实际路由
  - name: cheap-first
    match: { model: deepseek-v4-flash }   # 客户端用此名调用
    strategy: round_robin
    targets:
      - { provider: deepseek, model: deepseek-v4-flash }

  - name: smart-route                    # 语义分级
    match: { model: auto }
    strategy: semantic
    targets: []
    semantic_rules:
      - complexity: simple
        target: { provider: doubao, model: doubao-seed-2-0-mini-260428 }
      - complexity: complex
        target: { provider: doubao, model: doubao-seed-2-0-pro-260215 }

rate_limit:             # 限流(可选)
  enabled: false
  per_key: 60           # 每个 API key 每分钟 token 数
  per_model: 100        # 每个上游模型每分钟 token 数

cache:                  # 缓存(可选)
  enabled: false
  backend: memory       # memory | redis
  ttl: 1h
  strategy: exact       # exact | semantic
  max_size: 1000
  threshold: 0.85       # 语义匹配相似度阈值
  redis_addr: localhost:6379

tracing:                # OTel(可选)
  enabled: false
  exporter: otlp        # otlp | stdout
  service_name: ai-gateway
  sample_ratio: 1.0
```

## 路由策略

每条 route 一种策略,适配不同业务场景:

- **`round_robin`** — 简单轮询,适合同质多 key 厂商(如 SiliconFlow)。
- **`weighted`** — 按配置权重分配,人工指定流量比例。
- **`latency`** — 按最近窗口内实测 p99 延迟反比加权,快的厂商自然吸收更多流量,无需人工调权。失败率超阈值的厂商自动跳过。
- **`semantic`** — 根据 prompt 长度、是否含代码块、是否含多步指令等启发式,判 `simple` / `complex`,分别路由到不同档位的模型(便宜模型应付闲聊,贵模型处理复杂任务)。
- **`fallback`** — 按顺序尝试多个 target,任一成功即返回,适合主备切换。

不同策略的 target 配置字段不完全一致,详见 `internal/router/`。

## 韧性层:为什么这样设计

`熔断 → 重试 → 单飞 → 路由` 四件套的层叠顺序是反复调出来的:

- **重试发生在熔断内部**。每个用户的请求对熔断器只算 1 次调用,内部的重试不会让熔断器瞬间被击穿。
- **单飞发生在重试之外**。N 个并发相同请求合并成 1 次上游调用,响应共享给所有等待者,避免 LLM 厂商的并发上限被打爆。
- **429 单独识别**。429 是上游配额问题,会走 `Retry-After` 退避;4xx 其它码是客户端错,既不重试也不计入熔断。
- **流式单独走链**。流一旦发出去第一字节就不能撤回,所以重试只能发生在"开流前";开流后单飞+熔断继续生效。

## 缓存

- `exact` — 对完整请求体做哈希,精确匹配。
- `semantic` — 基于 prompt 嵌入向量做相似度匹配,阈值 `threshold` 可调(默认 0.85)。
- **流也能命中**。命中缓存的流会被切成 64 字符的 chunk 序列重放给客户端,不是一把塞回去。
- `memory` 后端是带 LRU 上限的进程内 map;`redis` 后端可多实例共享。

## 限流

令牌桶,按两个独立维度:

- `per_key` — 按调用方 API key 限流(配合 `auth.keys` 一起用)。
- `per_model` — 按上游模型限流(保护额度紧张的模型)。

桶是 lazy 创建,后台 goroutine 周期清理长时间未用的 key,避免高基数 key 撑爆内存。

## 鉴权

`auth.keys` 里列出的 key 是调用方需要出示的 key(放在请求头 `Authorization: Bearer <key>` 里),与厂商 API key **不是同一个**。前者是控制谁能进网关,后者是网关去调厂商时用的。

密钥校验使用 SHA-256 前缀索引 + `subtle.ConstantTimeCompare`,对每个候选项用常数时间比较,防计时侧信道。

## 可观测

### Prometheus 指标

`GET /metrics` 暴露,常用指标:

- `gateway_in_flight_requests` — 在飞请求数
- `gateway_cache_hits_total{result="hit"|"miss"}` — 缓存命中/未命中
- `gateway_upstream_errors_total{provider="..."}` — 上游错误计数
- `gateway_breaker_short_circuit_total{provider="..."}` — 熔断器跳过的请求数
- `gateway_breaker_state_transitions_total{provider="...",to="open"|"half_open"|"closed"}` — 熔断器状态切换
- `gateway_retry_attempts_total{provider="..."}` — 重试次数
- `gateway_coalesced_requests_total` — 被单飞合并的请求数

### 健康检查与状态

- `GET /admin/health` — 进程健康
- `GET /admin/routes` — 当前加载的所有路由
- `GET /admin/cache` — 缓存命中率、容量

### 链路追踪

开启 `tracing.enabled: true` 并配置 OTLP endpoint 后,每次请求会生成如下 span:

```
gateway.handle
├── route.select
└── provider.call
    ├── retry attempt 1
    ├── retry attempt 2
    └── ...
```

W3C `traceparent` 会从入站请求透传,方便和上游服务串起来。

### 成本估算

`internal/observer/cost.go` 内置主流模型每百万 token 的价格,请求结束时会算一次并打到日志。生产上建议把它接到 Prometheus `gateway_cost_usd_total` 自定义指标或者直接吐给计费系统。

## 端到端示例

```bash
# 1) 启动
./bin/gateway -config config/gateway.yaml &

# 2) 健康检查
curl http://localhost:8080/admin/health
# {"status":"ok"}

# 3) 走 round_robin 路由
curl -X POST http://localhost:8080/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"deepseek-v4-flash","messages":[{"role":"user","content":"hi"}]}'

# 4) 走 semantic 路由(simple → mini,complex → pro)
curl -X POST http://localhost:8080/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"auto","messages":[{"role":"user","content":"用 Python 写一个快速排序"}]}'

# 5) 流式
curl -N -X POST http://localhost:8080/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{"model":"deepseek-v4-flash","stream":true,"messages":[{"role":"user","content":"讲个笑话"}]}'
```

## 项目结构

```
ai-gateway/
├── cmd/gateway/main.go         # 入口
├── config/                     # YAML 加载与热重载
├── internal/
│   ├── provider/               # OpenAI / Claude 适配器
│   ├── router/                 # 5 种路由策略
│   ├── breaker/                # 熔断器
│   ├── retry/                  # 退避重试
│   ├── cache/                  # 精确+语义缓存,内存+Redis,单飞合并
│   ├── limiter/                # 令牌桶
│   ├── middleware/             # 鉴权 / 限流 / 缓存
│   ├── observer/               # 请求级观测与成本
│   ├── metrics/                # Prometheus 指标注册
│   ├── tracing/                # OTel 包装
│   └── server/                 # HTTP server 与 admin 端点
├── config/gateway.yaml         # 配置样例
├── go.mod / go.sum
└── README.md
```

各目录都配了 `*_test.go` 与 `bench_test.go`,跑基准:

```bash
go test ./...
go test -bench=. -benchmem ./...
```

## 上手建议

1. **先关鉴权、关限流、关缓存、关追踪,跑通基本路由**。再逐个打开观察指标变化。
2. **生产第一个要打开的是熔断 + 重试**,这两个是保命用。配置好后用 `wrk` 或 `k6` 故意把一个上游打挂,验证熔断器会按预期 open / half-open / close。
3. **第二个打开的是缓存**。先开 `exact`,看命中率是否符合预期(典型 30~60%),再考虑切到 `semantic`。
4. **限流放到最后**。`per_key` 给内部业务方分配,`per_model` 给单厂商配额保护,数值根据实际账单调。

## 路线图

- [ ] 厂商适配器插件化,允许运行时注册新厂商
- [ ] 缓存命中时按命中位置反算成本节省,产出 `cache_savings_usd_total` 指标
- [ ] 路由层支持基于工具调用的策略(如 `function_call` 强制路由到特定模型)
- [ ] 内置 prompt 模板与变量管理

## 贡献

提 issue / PR 都可以,提交前跑 `go vet ./...` 与 `go test ./...`。

## 许可

本仓库未附带 license 文件,默认保留所有权利。如需在自有产品中使用,请先与作者沟通授权。
