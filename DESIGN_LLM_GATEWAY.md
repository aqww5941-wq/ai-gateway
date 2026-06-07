# LLM API Gateway 设计文档

## 一、整体架构

```
                          ┌─────────────────────────┐
                          │     LLM API Gateway      │
                          │                          │
   Client (OpenAI SDK) ──►│  /v1/chat/completions    │
                          │  /v1/embeddings          │
                          │                          │
                          │  ┌──────────────────┐    │
                          │  │   Middleware Chain │    │
                          │  │  Auth → RateLimit  │    │
                          │  │  → Cache → Route   │    │
                          │  │  → Transform       │    │
                          │  └──────────────────┘    │
                          │                          │
                          │  ┌──────────────────┐    │
                          │  │  Provider Adapter  │    │
                          │  │  OpenAI / Claude / │    │
                          │  │  DeepSeek / GLM    │    │
                          │  └──────────────────┘    │
                          │                          │
                          │  ┌──────────────────┐    │
                          │  │  Observer          │    │
                          │  │  Metrics / Trace /  │    │
                          │  │  Cost Tracker      │    │
                          │  └──────────────────┘    │
                          └─────────────────────────┘
```

## 二、核心设计原则

### 2.1 统一入口，多模型透明切换

- 对外暴露标准 OpenAI API 格式（`/v1/chat/completions`）
- 客户端只需改 `base_url` 和 `api_key`，代码零改动
- 请求进来后，网关根据路由规则决定调用哪个后端模型
- 响应统一转回 OpenAI 格式返回

### 2.2 请求生命周期

```
Request → Auth(API Key 校验)
       → RateLimit(令牌桶/滑动窗口)
       → CacheCheck(精确匹配 + 语义相似匹配)
       → Router(根据规则选 Provider+Model)
       → ProviderCall(调用真实的 LLM API)
       → ResponseTransform(统一格式化)
       → CacheStore(异步写缓存)
       → Metrics(记录延迟/Token/成本)
       → Response
```

每一步都是一个独立的 middleware，可插拔、可组合。

## 三、模块设计

### 3.1 Config 模块 — 配置中心

```yaml
# gateway.yaml
server:
  port: 8080
  read_timeout: 30s
  write_timeout: 120s   # LLM 响应可能很慢

providers:
  - name: openai
    type: openai
    api_key: ${OPENAI_API_KEY}
    base_url: https://api.openai.com/v1
    models:
      - gpt-4o
      - gpt-4o-mini
    priority: 1
    max_retries: 2
    timeout: 60s

  - name: deepseek
    type: openai           # DeepSeek 也是 OpenAI 兼容的
    api_key: ${DEEPSEEK_API_KEY}
    base_url: https://api.deepseek.com/v1
    models:
      - deepseek-chat
      - deepseek-reasoner
    priority: 2

routes:
  - name: cheap-first
    match:
      model: gpt-4o-mini    # 请求声明要 gpt-4o-mini
    strategy: weighted       # weighted | round_robin | fallback | semantic
    targets:
      - provider: openai
        model: gpt-4o-mini
        weight: 50
      - provider: deepseek
        model: deepseek-chat
        weight: 50

  - name: smart-route
    match:
      model: auto            # 客户端传 model=auto 触发语义路由
    strategy: semantic
    semantic_rules:
      - complexity: simple
        target: { provider: deepseek, model: deepseek-chat }
      - complexity: complex
        target: { provider: openai, model: gpt-4o }

rate_limit:
  enabled: true
  per_key: 60/minute          # 每个 API Key 每分钟 60 次
  per_model: 100/minute       # 每个模型每分钟 100 次

cache:
  enabled: true
  backend: memory              # memory | redis
  ttl: 1h
  strategy: exact              # exact | semantic
  max_size: 1000               # 最多缓存 1000 条
```

### 3.2 Router 模块 — 路由策略

```go
// 路由接口，方便扩展新策略
type RoutingStrategy interface {
    Select(ctx context.Context, req *ChatRequest, targets []Target) (*Target, error)
}
```

**四种内置策略：**

| 策略 | 说明 | 适用场景 |
|------|------|---------|
| `weighted` | 按权重随机分配 | 负载均衡、省钱 |
| `round_robin` | 轮询分配 | 均匀打散请求 |
| `fallback` | 优先级链，失败自动降级 | 高可用 |
| `semantic` | 分析请求复杂度，自动选模型 | 智能省钱 |

语义路由的核心思路：对请求内容做快速分类（问题长度、是否含代码、是否多轮推理），简单的分给小模型，复杂的给大模型。这个判断可以用规则搞定，不需要再调一次 LLM。

```go
func classifyComplexity(req *ChatRequest) Complexity {
    // 规则1: 消息总长度 > 2000 tokens → complex
    // 规则2: 包含 system message 且要求多步骤推理 → complex
    // 规则3: user message 包含代码块 → complex
    // 默认: simple
}
```

### 3.3 Provider 模块 — 模型适配

```go
// Provider 接口
type LLMProvider interface {
    Name() string
    ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    ChatCompletionStream(ctx context.Context, req *ChatRequest) (<-chan *StreamChunk, error)
    ListModels() []string
}
```

**需要实现的 Provider：**

1. **OpenAIProvider** — 原生 OpenAI API
2. **OpenAICompatProvider** — 通用 OpenAI 兼容协议（覆盖 DeepSeek、GLM、通义千问等大多数国产模型）
3. **ClaudeProvider** — Anthropic Messages API（需要做格式转换，这是体现深度的点）

重点是 **请求/响应格式的统一转换**：不管后端是 OpenAI 格式还是 Anthropic 格式，网关对外永远输出 OpenAI Chat Completion 格式。

### 3.4 Middleware 模块 — 插件链

```go
type Middleware func(next http.Handler) http.Handler

// 注册顺序即执行顺序
func buildPipeline() http.Handler {
    h := http.HandlerFunc(handleChatCompletion)
    h = withAuth(h)
    h = withRateLimit(h)
    h = withCache(h)
    h = withRouting(h)
    h = withMetrics(h)
    h = withRecovery(h)    // panic recover 放最外层
    return h
}
```

### 3.5 Cache 模块 — Prompt 缓存

这是**面试亮点**。LLM 调用的特点是：很多请求有相同的 system prompt，这就能缓存。

```go
type CacheBackend interface {
    Get(key string) (*ChatResponse, bool)
    Set(key string, resp *ChatResponse, ttl time.Duration)
}

// 缓存 Key 生成
func cacheKey(req *ChatRequest) string {
    // 把 messages + model + temperature + top_p 做 hash
    // system prompt 相同时 key 就相同
    return hash(req.Messages, req.Model, req.Temperature)
}
```

进阶版可以做到**语义缓存**：不要求完全匹配，用 embedding 算相似度，相似度 > 0.95 就命中缓存。这个比较难但有区分度。

### 3.6 RateLimit 模块 — 流量控制

```go
type RateLimiter interface {
    Allow(key string) bool
}

// 两种实现：
// 1. TokenBucketLimiter   — 令牌桶，本地内存，无外部依赖
// 2. SlidingWindowLimiter — 滑动窗口，Redis 实现，支持分布式
```

MVP 阶段用令牌桶就行（`golang.org/x/time/rate` 自带），不需要 Redis。

### 3.7 Observer 模块 — 可观测性

```go
type Observer struct {
    reqID       string
    startTime   time.Time
    model       string
    provider    string
    promptTokens  int
    completionTokens int
    cost          float64
    latency       time.Duration
    cacheHit      bool
    status        int
}
```

每次请求完成后，打印一条结构化日志：

```json
{
  "req_id": "abc123",
  "model": "gpt-4o",
  "provider": "openai",
  "tokens": {"prompt": 200, "completion": 50},
  "cost": 0.00125,
  "latency_ms": 1200,
  "cache_hit": false,
  "status": 200
}
```

这是面试时能讲的东西：你如何做成本追踪、如何发现哪个模型性价比最高、如何做异常检测。

### 3.8 Stream 处理 — 流式透传

这是整个项目里**最体现 Go 并发功底**的部分。

```
Client ←──[SSE stream]── Gateway ←──[SSE stream]── LLM Provider
                            │
                            ├── 转发 chunk 给客户端
                            ├── 同时收集完整响应（用于缓存）
                            └── 同时做 token 计数（用于计费）
```

Go 实现：一个 goroutine 负责读上游 stream，通过 channel 广播给多个消费者（一个写给客户端、一个收集完整响应、一个算 token）。

```go
func (g *Gateway) handleStream(ctx context.Context, upstream <-chan *StreamChunk) {
    clientCh := make(chan *StreamChunk, 10)
    collectorCh := make(chan *StreamChunk, 10)

    // 上游读到的 chunk 广播给两个下游
    go func() {
        defer close(clientCh)
        defer close(collectorCh)
        for chunk := range upstream {
            clientCh <- chunk
            collectorCh <- chunk
        }
    }()

    // 消费者1: 写给 HTTP 客户端
    go g.writeSSE(w, clientCh)

    // 消费者2: 收集完整响应
    go g.collectResponse(collectorCh)
}
```

## 四、目录结构

```
llm-gateway/
├── cmd/
│   └── gateway/
│       └── main.go              # 入口
├── config/
│   ├── config.go                # 配置解析
│   └── gateway.yaml             # 示例配置
├── internal/
│   ├── server/
│   │   └── server.go            # HTTP Server
│   ├── router/
│   │   ├── router.go            # Router 主逻辑
│   │   ├── strategies.go        # 路由策略实现
│   │   └── semantic.go          # 语义路由
│   ├── provider/
│   │   ├── provider.go          # Provider 接口
│   │   ├── openai.go            # OpenAI Provider
│   │   ├── openai_compat.go     # OpenAI 兼容 Provider
│   │   └── claude.go            # Claude Provider
│   ├── middleware/
│   │   ├── auth.go              # API Key 校验
│   │   ├── ratelimit.go         # 限流
│   │   ├── cache.go             # 缓存
│   │   ├── metrics.go           # 指标收集
│   │   └── recovery.go          # Panic 恢复
│   ├── cache/
│   │   └── memory.go            # 内存缓存实现
│   ├── limiter/
│   │   └── token_bucket.go      # 令牌桶限流器
│   └── observer/
│       ├── observer.go          # 请求观测
│       └── cost.go              # 成本计算（各模型价格表）
├── go.mod
├── go.sum
└── Makefile
```

8-12 个文件，500-800 行核心代码，精而不杂。

## 五、分阶段实施计划

### Phase 1: MVP（核心链路跑通，2-3天）

- [ ] Config 模块：YAML 解析
- [ ] Provider 接口 + OpenAIProvider 实现
- [ ] Router：只做 weighted 和 round_robin
- [ ] HTTP Handler：`POST /v1/chat/completions`
- [ ] 验证：`curl` 通过网关调用 OpenAI，成功返回

### Phase 2: 生产特性（开始有区分度，2-3天）

- [ ] RateLimit：令牌桶限流
- [ ] Cache：精确匹配缓存
- [ ] Auth：API Key 白名单校验
- [ ] Fallback 路由：失败自动切备用模型
- [ ] Claude Provider：Anthropic 格式转换

### Phase 3: 面试亮点（2-3天）

- [ ] Stream 透传 + 多消费者 goroutine
- [ ] 语义路由：根据请求复杂度选模型
- [ ] Observer + 成本追踪
- [ ] Graceful shutdown（优雅关停）
- [ ] 结构化日志 + Prometheus metrics

### Phase 4: 锦上添花（有时间就做）

- [ ] 配置热更新（fsnotify）
- [ ] Redis 缓存后端
- [ ] 语义缓存（embedding 相似匹配）
- [ ] Admin API：查看路由状态、缓存命中率

## 六、面试回答话术参考

**"为什么做这个项目？"**
> 我在上一个项目里深度使用了多个 LLM API（OpenAI、DeepSeek），踩了直接调用的坑：切模型要改代码、出故障没有降级、相同 prompt 反复调用浪费钱。所以用 Go 做了这个网关，把这些问题统一解决。

**"为什么用 Go？"**
> LLM 调用的瓶颈是并发和 I/O，每一个请求到网关后要做限流、缓存、路由、转发、流式透传，中间有大量的并发读写和 channel 通信。Go 的 goroutine 和 channel 特别适合这种场景，代码量比 Java 少很多，性能比 Python 高很多。

**"技术难点？"**
> 核心难点有三个：一是流式响应的多路复用——上游一个 stream 要同时喂给 HTTP 客户端、缓存收集器和 token 计数器，用 goroutine+channel 的扇出模式解决；二是多协议的格式统一，Claude Messages API 和 OpenAI Chat API 的数据结构差异很大，消息角色、多轮对话、system prompt 的位置都需要映射；三是缓存策略的设计，精确匹配命中率低，语义匹配成本高，折中方案是用 hash + 部分匹配。

## 七、技术选型

| 需求 | 选型 | 理由 |
|------|------|------|
| HTTP 框架 | 标准库 `net/http` | Go 标准库足够好，不需要 Gin/Chi 等第三方依赖，越少依赖面试越加分 |
| 路由 | `http.ServeMux`（Go 1.22+） | 原生支持路径参数 |
| 配置 | `gopkg.in/yaml.v3` | YAML 可读性好，配置项多时比环境变量清晰 |
| 限流 | `golang.org/x/time/rate` | 官方扩展包，令牌桶实现 |
| 日志 | `log/slog`（Go 1.21+） | 结构化日志，标准库自带 |
| LLM SDK | 自己写 HTTP 调用 | 不用第三方 SDK，展示你对协议的理解 |
| Redis | MVP 阶段不需要 | 内存缓存 + 令牌桶先跑通，Redis 作为扩展 |
| 测试 | 标准库 `testing` | 轻量 |

核心原则：**能用标准库就不用第三方库**。面试官看的是一个依赖少的 Go 项目，印象分会更高。
