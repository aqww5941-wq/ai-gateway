# LLM API Gateway — 项目指南

## 一、项目概述

LLM API Gateway 是一个用 Go 编写的轻量级 LLM API 网关，对外暴露标准 OpenAI API 格式，内部可以透明地路由到多个 LLM 提供商（OpenAI、DeepSeek、Claude 等），支持缓存、限流、认证、语义路由、流式透传、成本追踪等功能。

**核心价值**：客户端只需改 `base_url` 和 `api_key`，代码零改动即可享受多模型切换、自动降级、缓存加速等能力。

## 二、技术栈

| 类别 | 选型 | 说明 |
|------|------|------|
| 语言 | Go 1.26 | goroutine + channel 天然适合高并发 I/O 场景 |
| HTTP 框架 | `net/http` 标准库 | Go 1.22+ 原生支持路径参数，无需 Gin/Chi |
| 配置解析 | `gopkg.in/yaml.v3` | YAML 可读性好，支持 `${ENV}` 环境变量展开 |
| 限流 | `golang.org/x/time/rate` | 官方扩展包，令牌桶实现 |
| 日志 | `log/slog` 标准库 | Go 1.21+ 结构化日志 |
| Redis 客户端 | `github.com/redis/go-redis/v9` | 可选，缓存后端和分布式限流 |
| 配置热更新 | `github.com/fsnotify/fsnotify` | 监听文件变更 |
| 测试 | `testing` 标准库 + `httptest` | 轻量无外部依赖 |

**核心原则：能用标准库就不用第三方库。** 整个项目仅 5 个直接依赖。

## 三、项目结构

```
ai-gateway/
├── cmd/
│   └── gateway/
│       └── main.go                  # 入口：组装 providers、router、server
├── config/
│   ├── config.go                    # YAML 配置解析（支持 ${ENV} 展开）
│   ├── gateway.yaml                 # 示例配置
│   └── reloader.go                  # fsnotify 配置热更新
├── internal/
│   ├── server/
│   │   ├── server.go                # HTTP Server 主逻辑 + 请求处理 + 流式处理
│   │   ├── admin.go                 # Admin API（健康检查、路由查看、缓存统计）
│   │   └── mock_test.go            # 集成测试（mock LLM 服务器）
│   ├── router/
│   │   ├── router.go                # Router 主逻辑 + 四种策略注册
│   │   ├── strategies.go           # （策略实现在同目录各文件中）
│   │   ├── semantic.go              # 语义路由：请求复杂度分析 + 自动选模型
│   │   ├── fallback.go              # 降级路由：优先级链失败自动切换
│   │   ├── router_test.go           # 路由策略单元测试
│   │   └── semantic_test.go         # 语义路由单元测试
│   ├── provider/
│   │   ├── provider.go              # LLMProvider 接口 + 通用数据结构
│   │   ├── openai.go                # OpenAI Provider（含 SSE 流式读取）
│   │   └── claude.go                # Claude Provider（Anthropic Messages API 格式转换）
│   ├── middleware/
│   │   ├── auth.go                  # API Key 认证（Bearer Token + 常数时间比较）
│   │   ├── ratelimit.go             # 限流中间件（按 API Key + 按模型）
│   │   ├── cache.go     # 观察者中间件（请求计时 + 结构化日志）
│   │   └── auth_test.go            # 认证单元测试
│   ├── cache/
│   │   ├── backend.go               # CacheBackend 接口定义
│   │   ├── memory.go                # 内存缓存实现（SHA256 Key + TTL + LRU 淘汰）
│   │   ├── redis.go                 # Redis 缓存实现
│   │   ├── semantic.go              # 语义缓存（词袋嵌入 + 余弦相似度匹配）
│   │   ├── memory_test.go           # 内存缓存测试
│   │   └── semantic_test.go         # 语义缓存测试
│   ├── limiter/
│   │   ├── token_bucket.go          # 令牌桶限流器（golang.org/x/time/rate）
│   │   └── token_bucket_test.go     # 限流器测试
│   └── observer/
│       ├── observer.go              # 请求观测器（生成 req_id + 结构化日志）
│       └── cost.go                  # 成本计算（各模型价格表，按 1M tokens 计价）
├── go.mod
├── go.sum
├── DESIGN_LLM_GATEWAY.md            # 原始设计文档
└── PROJECT_GUIDE.md                 # 本文件
```

**代码量**：约 25 个 `.go` 文件，核心代码 ~1200 行，精而不杂。

## 四、架构设计

### 4.1 整体架构图

```
                          ┌─────────────────────────┐
                          │     LLM API Gateway      │
                          │                          │
   Client (OpenAI SDK) ──►│  /v1/chat/completions    │
                          │                          │
                          │  ┌──────────────────┐    │
                          │  │  Middleware Chain  │    │
                          │  │  Auth → RateLimit   │    │
                          │  │  → Metrics          │    │
                          │  └──────────────────┘    │
                          │           │              │
                          │           ▼              │
                          │  ┌──────────────────┐    │
                          │  │  Route + Cache     │    │
                          │  │  (Cache Check →     │    │
                          │  │   Router Select →   │    │
                          │  │   Provider Call →   │    │
                          │  │   Cache Store)     │    │
                          │  └──────────────────┘    │
                          │           │              │
                          │           ▼              │
                          │  ┌──────────────────┐    │
                          │  │  Provider Adapter  │    │
                          │  │  OpenAI / Claude    │    │
                          │  └──────────────────┘    │
                          │           │              │
                          │           ▼              │
                          │  ┌──────────────────┐    │
                          │  │  Observer          │    │
                          │  │  Log + Cost Calc   │    │
                          │  └──────────────────┘    │
                          └─────────────────────────┘
```

### 4.2 请求生命周期

```
Request
  │
  ├─ 1. Auth Middleware
  │     └─ 校验 Authorization: Bearer <key>（常数时间比较防时序攻击）
  │
  ├─ 2. RateLimit Middleware
  │     └─ 令牌桶：按 API Key 限流 + 按模型限流
  │
  ├─ 3. Metrics Middleware（最外层包装器）
  │     └─ 记录 method、path、status、latency
  │
  ├─ 4. Handler: handleChatCompletion
  │     │
  │     ├─ 4a. 非流式 (stream=false)
  │     │   ├─ Cache Check（精确匹配 SHA256 → 语义匹配余弦相似度）
  │     │   ├─ Router.Route()：根据 model 字段匹配路由规则
  │     │   ├─ Provider.ChatCompletion()：调用真实 LLM API
  │     │   ├─ Cache Store（异步写入缓存）
  │     │   └─ Observer.Finalize()：日志 + 成本计算
  │     │
  │     └─ 4b. 流式 (stream=true)
  │         ├─ Router.Route() → Provider.ChatCompletionStream()
  │         ├─ Fan-out：一个 goroutine 读上游 → 广播给两个 channel
  │         ├─ Consumer 1：写 SSE 给 HTTP 客户端
  │         ├─ Consumer 2：收集完整响应 → 写入缓存
  │         └─ Observer.Finalize()
  │
  └─ 5. Recovery Middleware（最外层）
        └─ panic recover，返回 500
```

### 4.3 核心模块详解

#### 4.3.1 Provider 模块 — 多模型适配

所有 Provider 实现统一接口：

```go
type LLMProvider interface {
    Name() string
    ChatCompletion(ctx, req) (*ChatResponse, error)
    ChatCompletionStream(ctx, req) (<-chan *StreamChunk, error)
    SupportedModels() []string
}
```

**两种 Provider 实现**：

| Provider | 后端 API | 关键逻辑 |
|----------|---------|---------|
| `OpenAIProvider` | OpenAI `/v1/chat/completions` | 直接透传，也兼容 DeepSeek 等 OpenAI 兼容 API |
| `ClaudeProvider` | Anthropic `/v1/messages` | 需要**格式转换**：将 system message 提取到顶层 `system` 字段，响应时将 `content[]` 拼接为字符串，映射 `stop_reason` |

#### 4.3.2 Router 模块 — 四种路由策略

```go
// 路由匹配：根据请求中的 model 字段找到对应路由规则
// 路由规则按 Match.Model 建立索引（map[string]*routeEntry）
// 例如：请求 model="gpt-4o-mini" → 命中 cheap-first 规则
```

| 策略 | 实现 | 适用场景 |
|------|------|---------|
| `round_robin` | atomic counter 轮询 | 均匀打散请求 |
| `weighted` | 加权随机（累积概率） | 按比例分流，省钱 |
| `fallback` | 返回优先级链，失败自动切下一个 | 高可用 |
| `semantic` | 分析请求复杂度，简单→小模型，复杂→大模型 | 智能省钱 |

**语义路由的复杂度判定规则（纯规则，不调 LLM）**：

```go
func classifyComplexity(req) Complexity {
    // 规则1: 消息总长度 > 2000 字符 → complex
    // 规则2: system message 含多步骤推理关键词 → complex
    // 规则3: 包含代码块 → complex
    // 默认: simple
}
```

#### 4.3.3 Cache 模块 — 两级缓存

**Key 生成**：`SHA256(model + temperature + messages[].role + messages[].content)`

| 缓存层 | 匹配方式 | 实现 |
|--------|---------|------|
| 精确缓存 | SHA256 哈希完全匹配 | `MemoryCache` / `RedisCache` |
| 语义缓存 | 词袋嵌入 + 余弦相似度 ≥ 阈值 | `SemanticCache` |

语义缓存使用 40 个编程/LLM 常用词构建 TF 向量，计算余弦相似度。这是轻量级近似方案，避免了调 embedding API 的成本。

**缓存后端**：
- `memory`：内存 map + 互斥锁 + TTL 过期 + 超过 maxSize 时随机淘汰一条
- `redis`：Redis 存储，适合分布式部署

#### 4.3.4 流式处理 — goroutine 扇出模式

```
上游 LLM Provider (SSE stream)
        │
        ▼
  broadcast goroutine
  (读上游 chunk → 同时发送到两个 channel)
        │
        ├─► clientCh ──► SSE Writer goroutine
        │                (写 HTTP Response, 实时 flush)
        │
        └─► collectorCh ──► Main goroutine
                         (收集完整响应 → 存缓存 + 计 token)
```

三个 goroutine 通过 buffered channel 通信，128 字节缓冲区避免阻塞。

#### 4.3.5 Observer 模块 — 可观测性

每次请求生成一个 8 字节随机 hex ID，记录：

```json
{
  "req_id": "a1b2c3d4e5f6g7h8",
  "model": "gpt-4o",
  "provider": "openai",
  "tokens": {"prompt": 200, "completion": 50},
  "cost": 0.00125,
  "latency_ms": 1200,
  "cache_hit": false,
  "status": 200
}
```

成本计算内置了主流模型的价格表（按每 1M tokens 美元计价）。

#### 4.3.6 配置热更新

使用 `fsnotify` 监听 YAML 文件变更，检测到写入事件后重新加载配置，通过 `Server.Reload()` 原子替换 Router 和 Providers，无需重启服务。

## 五、测试方法

### 5.1 单元测试（不需要 API Key）

项目中已有 7 个测试文件，覆盖了核心模块：

```bash
# 跑所有测试
cd /home/aqww/ai-gateway && go test ./internal/... -v

# 只跑某个包的测试
go test ./internal/router/ -v
go test ./internal/cache/ -v
go test ./internal/limiter/ -v
go test ./internal/middleware/ -v
go test ./internal/server/ -v

# 带覆盖率
go test ./internal/... -cover
```

**测试覆盖范围**：

| 测试文件 | 测试内容 |
|---------|---------|
| `server/mock_test.go` | 完整链路集成测试：非流式请求、缓存命中、流式透传（均用 httptest mock LLM） |
| `router/router_test.go` | 加权路由、轮询路由、降级路由策略 |
| `router/semantic_test.go` | 语义路由复杂度分类（简单/复杂判定） |
| `cache/memory_test.go` | 缓存读写、TTL 过期、Key 生成 |
| `cache/semantic_test.go` | 语义缓存相似度匹配 |
| `limiter/token_bucket_test.go` | 令牌桶限流 Allow/Reject |
| `middleware/auth_test.go` | API Key 认证通过/拒绝 |

### 5.2 启动网关进行真实测试（需要 API Key）

#### 步骤 1：设置环境变量

```bash
export OPENAI_API_KEY=sk-your-openai-key
export DEEPSEEK_API_KEY=your-deepseek-key
# Claude 可选
export ANTHROPIC_API_KEY=sk-ant-your-key
```

#### 步骤 2：启动网关

```bash
cd /home/aqww/ai-gateway
go run ./cmd/gateway/ --config config/gateway.yaml
```

网关默认监听 `:8080`。

#### 步骤 3：curl 测试

**非流式调用**：
```bash
curl -s http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "用一句话介绍 Go 语言"}],
    "stream": false
  }' | jq .
```

返回示例：
```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion",
  "model": "gpt-4o-mini",
  "choices": [{
    "index": 0,
    "message": {
      "role": "assistant",
      "content": "Go 是 Google 开发的..."
    },
    "finish_reason": "stop"
  }],
  "usage": {
    "prompt_tokens": 15,
    "completion_tokens": 20,
    "total_tokens": 35
  }
}
```

**流式调用**：
```bash
curl -N http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "说个笑话"}],
    "stream": true
  }'
```

**测试语义路由**（传 `model: "auto"` 触发）：
```bash
# 简单问题 → 路由到 deepseek-chat（省钱）
curl -s http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"auto","messages":[{"role":"user","content":"1+1=?"}],"stream":false}' | jq .model

# 复杂问题 → 路由到 gpt-4o（保证质量）
curl -s http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"auto","messages":[{"role":"user","content":"请详细分析微服务架构和单体架构的优缺点，并给出实际场景建议"}],"stream":false}' | jq .model
```

**Admin API**：
```bash
# 健康检查
curl http://localhost:8080/admin/health

# 查看路由配置
curl http://localhost:8080/admin/routes | jq .

# 查看缓存统计（命中率、条目数等）
curl http://localhost:8080/admin/cache | jq .
```

### 5.3 用 OpenAI SDK 测试（Python 示例）

这是该网关的设计亮点 —— 客户端**零代码改动**：

```python
from openai import OpenAI

# 只需要把 base_url 指向网关，其他代码不变
client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="any-key"  # 网关 auth 未启用时随便填
)

# 非流式
response = client.chat.completions.create(
    model="gpt-4o-mini",
    messages=[{"role": "user", "content": "Hello"}]
)
print(response.choices[0].message.content)

# 流式
stream = client.chat.completions.create(
    model="gpt-4o-mini",
    messages=[{"role": "user", "content": "讲个故事"}],
    stream=True
)
for chunk in stream:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="")
```

### 5.4 开启缓存测试

编辑 `config/gateway.yaml`，将缓存启用：
```yaml
cache:
  enabled: true
  backend: memory
  ttl: 1h
  strategy: exact    # 或 semantic
  max_size: 1000
```

重启网关后，发送两个**完全相同**的请求，第二个请求会命中缓存，网关日志会显示 `cache hit`，且不会真正调用上游 LLM。

也可以查看 Admin API 确认缓存命中：
```bash
curl http://localhost:8080/admin/cache | jq '{hits, misses, hit_rate_pct}'
```

### 5.5 开启限流测试

```yaml
rate_limit:
  enabled: true
  per_key: 5     # 每分钟每 key 5 次（方便测试）
  per_model: 10
```

用脚本快速发送请求验证 429 响应：
```bash
for i in $(seq 1 10); do
  curl -s -o /dev/null -w "req $i: %{http_code}\n" \
    http://localhost:8080/v1/chat/completions \
    -H "Content-Type: application/json" \
    -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}],"stream":false}'
done
```

### 5.6 测试降级路由

修改路由为 `fallback` 策略，断开主 Provider，观察是否自动切到备用 Provider。

## 六、核心数据流

### 6.1 非流式请求完整调用链

```
1. POST /v1/chat/completions  {"model":"gpt-4o-mini", "messages":[...]}
2. Recovery → Metrics → Auth → RateLimit → Handler
3. Handler 解析 JSON → ChatRequest
4. Cache.Get(cacheKey(req)) → 命中则直接返回
5. Router.Route(req) → 匹配 "cheap-first" 规则 → WeightedStrategy.Select()
   → 按权重 50:50 随机选 openai/gpt-4o-mini 或 deepseek/deepseek-chat
6. providers["openai"].ChatCompletion(ctx, req)
   → POST https://api.openai.com/v1/chat/completions
   → 反序列化 ChatResponse
7. Cache.Set(cacheKey, resp)
8. Observer.Finalize() → 结构化日志 + 成本计算
9. JSON 编码返回给客户端
```

### 6.2 流式请求完整调用链

```
1. POST /v1/chat/completions  {"stream":true, ...}
2. 同样的 Middleware 链
3. Handler 检测 req.Stream == true → handleStreamCompletion()
4. Router.Route() → 选 Provider
5. Provider.ChatCompletionStream() → 返回 <-chan *StreamChunk
6. Launch broadcast goroutine:
   for chunk := range upstream {
       clientCh <- chunk    // 给 SSE writer
       collectorCh <- chunk // 给缓存收集器
   }
7. Launch SSE writer goroutine:
   for chunk := range clientCh {
       fmt.Fprintf(w, "data: %s\n\n", json.Marshal(chunk))
       flusher.Flush()      // 立即推给客户端
   }
8. Main goroutine 消费 collectorCh:
   - 拼接 fullContent
   - 收集 token 计数
9. 流结束后，构建完整 ChatResponse 写入缓存
10. Observer.Finalize()
```

## 七、关键设计决策

1. **为什么用标准库而不是 Gin/Chi？** — Go 1.22+ 的 `http.ServeMux` 已支持路径参数和方法路由，足够好用。减少依赖在面试中也是加分项。

2. **为什么不直接用 OpenAI SDK？** — 自己写 HTTP 调用可以展示对协议的理解，也方便支持 Claude 等非 OpenAI 格式的 Provider。

3. **为什么缓存用 SHA256 而不是 MD5？** — SHA256 碰撞概率极低，且 `crypto/sha256` 标准库直接可用。

4. **为什么流式处理用 goroutine 扇出？** — 上游一个 stream 需要同时喂给两个消费者（客户端 + 缓存收集），Go 的 channel 扇出是最自然的方案。

5. **为什么语义路由用规则而不是调 LLM？** — 调 LLM 分类会增加延迟和成本，规则分类在绝大多数场景下已经足够准确。

## 八、扩展指南

- **添加新 Provider**：实现 `LLMProvider` 接口，在 `createProviders()` 中注册
- **添加新路由策略**：实现 `RoutingStrategy` 接口，在 `NewRouter()` 中注册
- **添加新缓存后端**：实现 `CacheBackend` 接口，在 `New()` 中注册
- **添加新的复杂度判定规则**：修改 `classifyComplexity()` 函数
