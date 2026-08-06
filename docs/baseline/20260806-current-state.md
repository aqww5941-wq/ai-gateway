# AI Gateway 当前行为基线

> Task：Task 1——建立可复现的当前行为基线
>
> 采集日期：2026-08-06（Asia/Shanghai）
>
> 代码基线：`master@d058f55b96a8027fdf90fad43dc04f343473cbe2`
>
> 事实等级：Current Implementation Evidence；本文不代表 v3 目标已经实现

## 1. 范围与结论

本文固定重构前的工具链、构建、测试、HTTP 路由、配置、Reload、Store、Provider、SSE、错误与降级行为。后续 Task 修改这些行为时，必须引用本基线、说明契约变化并增加回归测试。

当前项目可以通过普通 Go Test、Vet 和 Build，但还不能称为可信工程基线：总语句覆盖率为 35.7%，Config、Store 和 Provider 的有效测试覆盖为 0%，Race 门禁不可执行，前端产物存在两个 Git 事实源，且若干安全、账务与流式失败采用 Fail Open 或 Best Effort。

本 Task 只记录事实，没有修复任何运行行为。

## 2. 复现环境

| 项目 | 当前值 |
| --- | --- |
| OS | Microsoft Windows 10.0.26200，windows/amd64 |
| PowerShell | 7.6.4 |
| Go | go1.26.5；`go.mod` 声明 go1.26.4 |
| CGO | `CGO_ENABLED=0`，PATH 中没有 gcc |
| Node.js | v22.14.0 |
| npm | 10.9.2 |
| Git | 2.54.0.windows.1 |
| 分支 | `master` |

仓库已有未提交状态只有用户删除的 v2 设计文档；Task 1 未恢复、暂存或修改该文件。

## 3. 可复现检查命令与结果

在仓库根目录执行：

```powershell
& 'C:\Program Files\Go\bin\go.exe' test -count=1 -cover ./...
& 'C:\Program Files\Go\bin\go.exe' vet ./...
& 'C:\Program Files\Go\bin\go.exe' build -o "$env:TEMP\ai-gateway-task1-gateway.exe" ./cmd/gateway
Set-Location web
npm exec tsc -- --noEmit
```

| 检查 | 结果 | 证据摘要 |
| --- | --- | --- |
| Go Test + Coverage | 通过 | 所有现有 Test 通过；总语句覆盖率 35.7% |
| Go Vet | 通过 | 无输出，退出码 0 |
| Go Build | 通过 | Windows/amd64 可执行文件输出到系统临时目录，未污染工作树 |
| TypeScript noEmit | 通过 | 现有 `node_modules` 下退出码 0 |
| Go Race | 未通过 | `go: -race requires cgo`；当前 `CGO_ENABLED=0` 且 gcc 不在 PATH |
| Frontend Vite Build | 未执行 | 当前 build 会写入被跟踪的 `web/dist`；产物治理属于 Task 2 |
| 真实 Provider Smoke | 未执行 | Task 1 不访问外部厂商，也不使用真实 Credential |

Race 是真实的未建立门禁，不记为“环境下已通过”。Task 8/M0 Exit Gate 必须提供可运行的 Race 方案或经设计批准的等价平台门禁。

## 4. 当前启动与进程边界

### 4.1 入口

- 进程入口：`cmd/gateway/main.go:15`。
- 默认配置：`config/gateway.yaml`；可通过 `-config <path>` 指定。
- 当前只有一个 `http.Server` 和一个监听端口，默认 `:8081`。
- 数据 API、管理 API、管理 SPA 和 Prometheus metrics 共用该 Server。
- `start.sh`/`stop.sh` 是 Unix Shell 脚本；Makefile 的复制与删除命令同样依赖 Unix 工具。

启动命令：

```powershell
& 'C:\Program Files\Go\bin\go.exe' run ./cmd/gateway -config config/gateway.yaml
```

当前示例配置引用多个 Provider 环境变量。缺失变量会先被展开为空字符串，随后 Provider 构造返回 `api_key is empty` 并导致进程退出。因此新环境不能在未配置全部示例 Provider Key 时直接启动默认配置。

### 4.2 生命周期

- Tracing 在其他组件前初始化；失败时进程退出。
- Provider、Router 或 Server 构造失败时进程退出。
- Config Watcher 创建失败只记录 Warn，网关继续运行但关闭热重载。
- SIGINT/SIGTERM 触发 `http.Server.Shutdown`，固定超时 5 秒。
- Server 固定 `ReadTimeout=30s`、`WriteTimeout=120s`，不读取示例 YAML 中的 `read_timeout`/`write_timeout` 字段。

入口证据：`cmd/gateway/main.go`、`internal/server/server.go:222-248`。

## 5. HTTP 路由与鉴权边界

### 5.1 实际 Middleware 顺序

Auth/Rate Limit 开启时，从外到内为：

```text
RateLimit -> Auth -> QuotaCheck -> InFlight Metrics -> Request Metrics
-> ConcurrencyLimiter -> Recovery -> ServeMux
```

`/admin/**` 进入 ServeMux 后再经过 `AdminOnly`。`/metrics` 和 `/admin/dashboard/**` 跳过 Auth；Quota 跳过所有 `/admin` 与 `/metrics`；RateLimit 和 ConcurrencyLimiter 没有同等系统路径豁免。

### 5.2 数据面与系统路由

| Method/Path | 鉴权 | 当前行为与入口 |
| --- | --- | --- |
| `POST /v1/chat/completions` | Auth 开启时需要 Bearer Key | 唯一模型调用入口；`Server.handleChatCompletion` |
| `GET /metrics` | 无 | Prometheus handler；同一业务端口 |
| `/admin/dashboard/**` | SPA 本身无鉴权 | 内嵌 React 资源；API 请求另行携带 Admin Key |

不存在 `/v1/responses`、独立 liveness/readiness 或独立控制面端口。

### 5.3 Legacy Admin

以下接口需要有效且 `role=admin` 的 Gateway Key：

- `GET /admin/health`
- `GET /admin/routes`
- `GET /admin/cache`

`/admin/health` 只返回静态 `{"status":"ok"}`，不检查 Store、Provider、配置或迁移状态，因此它不是 readiness。

### 5.4 Admin API v1

以下接口均位于相同端口，并由 Auth + AdminOnly 保护：

| Method/Path | 当前用途 |
| --- | --- |
| `GET /admin/api/v1/overview` | 进程内请求、缓存、错误和 singleflight 统计 |
| `GET /admin/api/v1/breakers` | Breaker 快照 |
| `GET /admin/api/v1/providers` | 配置 Provider、模型、Breaker 派生健康状态 |
| `GET /admin/api/v1/latency` | 路由延迟快照 |
| `GET /admin/api/v1/routes` | 当前路由配置 |
| `GET /admin/api/v1/cache` | 缓存统计与内存条目 |
| `GET /admin/api/v1/cache/entries/{key}` | 仅内存缓存支持的单条查询 |
| `GET /admin/api/v1/quotas` | SQLite Quota 快照；无 Store 时返回 disabled 空列表 |
| `GET /admin/api/v1/keys` | Key 列表，不返回 token hash |
| `POST /admin/api/v1/keys` | 创建 Key，并在响应中返回一次明文 Token |
| `PUT /admin/api/v1/keys/{id}` | 更新元数据或启停 Key |
| `DELETE /admin/api/v1/keys/{id}` | 删除 Key |
| `GET /admin/api/v1/audit-logs` | 按 key/limit/offset 查询审计 |
| `GET /admin/api/v1/filter` | PII Filter 当前配置与规则 |

管理 API 没有 OpenAPI 契约和统一错误 Envelope；成功大多是 JSON，失败多使用 `http.Error` 纯文本，部分错误直接包含内部 Store 错误字符串。

入口证据：`internal/server/admin.go:50-75`、`internal/server/admin_api.go`、`internal/middleware/admin.go`。

## 6. Chat Completions 当前契约

### 6.1 可表达字段

`provider.ChatRequest` 只包含：

- `model`
- `messages[]`，且 `content` 只能是字符串
- `temperature`
- `max_tokens`
- `stream`

当前领域模型无法表达 Tools、Tool Result、Reasoning、Structured Output、Multimodal、Citation 或 Provider-managed State。JSON Decoder 不拒绝未知字段，也没有统一必填字段校验。

### 6.2 Unary 链路

```text
2 MiB LimitReader + JSON Decode
-> PII Filter
-> Key Model Allowlist
-> Exact/Semantic Cache
-> Router
-> Singleflight
-> Breaker
-> Retry（最多 3 次）
-> Provider
-> PII Response Filter
-> Cache
-> Best-effort Usage Record/Audit
```

当前 Cache/Singleflight Key 不包含 Key/租户身份。Fallback Chain 会在任意非 Context 错误后继续下一个目标，包括 Provider 返回的非重试型 4xx；它不验证 Tools/Reasoning 等能力，因为当前模型无法表达这些需求。

### 6.3 当前 HTTP 错误映射

| 条件 | 当前状态 |
| --- | --- |
| JSON decode 失败 | 400，返回 Decoder 错误文本 |
| Key 不允许模型 | 403 |
| PII block | 422 |
| Router 无匹配 | 404 |
| Quota 已耗尽 | 429 |
| 上游 429 | 429 |
| Breaker Open | 503 |
| 其他上游/网络/协议错误 | 通常 502 |
| Store Auth 查询失败 | 500 |

上游错误对象最多保留 1 KiB Body，但 `http.Error(w, err.Error(), ...)` 会把该错误摘要返回客户端；当前没有安全、稳定的外部错误码模型。

入口证据：`internal/provider/provider.go`、`internal/server/server.go:313-628`。

## 7. SSE 当前行为边界

### 7.1 Provider 侧

- OpenAI-compatible Adapter 使用 `bufio.Scanner` 和空行切分 SSE，单事件上限 1 MiB。
- 一个 SSE event 出现多条 `data:` 时只逐行尝试解析，注释、event 名称和 typed error 没有领域表示。
- JSON 解析失败只记录 Warn 并跳过该事件。
- Scanner 错误只记录 Warn；错误不会通过 Channel 返回 Server。
- `[DONE]` 只作为结束标记，不形成 typed completion event。
- Provider `http.Client.Timeout` 是整个请求超时，默认 30 秒；它同样约束长流。

### 7.2 Gateway 侧

- 仅在“打开上游 Stream”失败且尚未写客户端时尝试 Fallback；单目标 Stream 不使用 Retry。
- 写 Header 后返回 200，并由 Handler goroutine独占 ResponseWriter。
- 上游 Channel 关闭后无论是正常结束、解析错误还是扫描错误，Gateway 都尝试发送 `data: [DONE]`。
- 客户端写失败会 Cancel Context，但循环退出后仍尝试写 `[DONE]`，随后可能缓存已收集内容并按成功路径记录 Usage/Audit。
- 上游 Stream 缺少 Usage 时 Token 以 0 记录；没有 `estimated` 状态。
- 缓存命中会把完整文本每 64 个 rune 重新切成伪流，并发送 `[DONE]`。

因此当前“HTTP 200 + `[DONE]`”不能证明上游 Stream 正常完成。该事实是后续 typed event、Commit Barrier 和账务状态机的关键回归基线。

入口证据：`internal/provider/openai.go`、`internal/server/server.go:658-981`。

## 8. Provider 当前边界

| Provider Type | 当前能力 | 已知限制 |
| --- | --- | --- |
| `openai` | `base_url + /chat/completions` 的 Unary/SSE 转发 | 只有文本 Chat 子集；厂商 dialect 无独立 Codec/能力矩阵；Base URL/Redirect 未受控 |
| `claude` | Messages Unary 文本转换 | 不支持 Streaming；只拼接文本 Content Block；Tool/Thinking/多模态丢失 |

方舟、DeepSeek、SiliconFlow 等在当前配置中都通过同一个 `openai` Type 接入；这只能证明请求形状相似，不能证明厂商语义完整适配。Qwen 没有当前独立实现。

Provider 包当前只有 Benchmark，没有 `Test*`；Coverage 为 0%。没有官方 Golden、SSE Error Replay、Conformance 或真实 Smoke 证据。

## 9. 配置与 Reload 当前边界

### 9.1 配置来源

- CLI `-config` 指向单个 YAML 文件。
- `os.ExpandEnv` 对整个文件执行环境变量替换。
- 不存在 Control Plane DB、Secret Manager 或版本化 Config Revision。
- YAML 使用 `yaml.Unmarshal`，未知字段被忽略。
- 缺失环境变量变成空字符串；没有统一 required/default/range/cross-reference 校验阶段。
- 部分默认值散落在 Server、Provider、Cache 等构造函数。

示例 YAML 包含固定演示 Gateway Key，并声明了 Go Struct 不接收的 server timeout 字段；这些字段当前被静默忽略。

### 9.2 Reload

fsnotify 监听配置文件的 Write/Create：

1. 读取新 Config。
2. `Reloader` 在调用 Server callback 前就把自己的 `cfg` 指针换成新值。
3. `Server.Reload` 构建新 Provider 和 Router，校验 Route target。
4. 成功时在一把 mutex 下替换 `s.config`、`s.router`、`s.providers`。

Reload 不重建或替换 HTTP Server/Middleware、Transport、Cache、Rate Limiter、Concurrency Limiter、Store、Auth、Filter 或 Tracing。Callback 失败时 `Reloader.Config()` 已指向新配置，而 Server 仍使用旧配置。部分请求路径读取 Server 字段时没有持有相同读锁；当前 Config/Reload 没有测试或 Race 证据。

入口证据：`config/config.go`、`config/reloader.go`、`internal/server/server.go:251-310`。

## 10. Store、Auth、Quota 与 Audit 当前边界

### 10.1 Store 与 Key

- SQLite 是唯一 Store；非内存路径启用 WAL，迁移使用启动时 `CREATE TABLE IF NOT EXISTS` 和逐字段兼容修改。
- Store 打开失败只记录 Warn，Server 继续运行；Key 管理与持久 Quota 不可用，Auth 回退到静态配置 Key。
- Gateway Key 使用随机 Token，数据库保存 SHA-256；校验时查询全部 active Key 并逐一常数时间比较。
- Auth identity 缓存按 Token 索引，每分钟清空刷新；已禁用 Key 可能在缓存清理前继续有效。
- 管理 SPA 当前把 Admin Token 保存在浏览器 `localStorage`。

### 10.2 Quota

- `CheckQuota` 与上游调用后的 `RecordUsage` 是分离操作，没有预占。
- Quota Check 失败时 Middleware 记录 Error 后继续请求，即 Fail Open。
- 每日用量行查询的任意错误都按“未使用 0 Token”处理，而不只处理 No Rows。
- Monthly Limit 被读取但没有参与拒绝判断。
- Usage 写入失败只记录 Error，不改变已成功返回的模型响应。

### 10.3 Audit

- Audit 写入是 Best Effort；失败只记日志，不影响请求结果。
- Admin API 的部分 Store 错误直接返回 `err.Error()`。
- Store、Migration、Quota 和 Audit 当前均无自动测试覆盖。

入口证据：`internal/store/**`、`internal/middleware/auth.go`、`internal/middleware/quota.go`、`internal/server/server.go:631-655`、`web/src/api.ts`。

## 11. 缓存、韧性与可观测当前边界

- Redis Cache 连接失败会记录 Warn 并自动回退到本机 Memory Cache。
- Breaker 隔离键是 Provider Name，不是 endpoint/model/region。
- Unary Retry 默认最多 3 次，处理网络错误和 408/429/502/503/504，并尊重 Retry-After。
- Stream 不使用 Retry；Fallback 只覆盖打开 Stream 失败。
- Audit 为 Best Effort；Quota Check Fail Open；这些降级没有统一健康状态。
- 已有 slog、Prometheus 和 OpenTelemetry，但诊断字段没有统一 request_id/config_revision/committed/adapter_revision。
- `/admin/api/v1/providers` 的 health 仅由 Breaker 状态派生，不是主动 Provider 健康检查。

## 12. 自动测试与覆盖率基线

仓库共有 69 个 `Test*`、21 个 `Benchmark*`、0 个 `Fuzz*`。包级结果：

| 包 | Test 文件 | 语句覆盖率 |
| --- | ---: | ---: |
| `cmd/gateway` | 0 | 0.0% |
| `config` | 0 | 0.0% |
| `internal/breaker` | 1 | 81.0% |
| `internal/cache` | 3 | 60.0% |
| `internal/filter` | 1 | 80.6% |
| `internal/limiter` | 1 | 70.0% |
| `internal/metrics` | 0 | 0.0% |
| `internal/middleware` | 1（另有 benchmark） | 24.4% |
| `internal/observer` | 0 | 0.0% |
| `internal/provider` | 0（只有 benchmark） | 0.0% |
| `internal/retry` | 1 | 83.6% |
| `internal/router` | 4（另有 benchmark） | 58.9% |
| `internal/server` | 2 | 39.1% |
| `internal/static` | 0 | 0.0% |
| `internal/store` | 0 | 0.0% |
| `internal/tracing` | 1 | 20.0% |

现有 Server 测试覆盖 Unary、Cache、SSE、缓存回放、Breaker、Retry、Singleflight 和部分状态映射，但没有 Admin API、Quota/Store、Reload、Provider Codec 错误传播、客户端断流和完整 Shutdown 保护。

前端 `package.json` 只有 dev/build/preview，没有 lint 或 test 脚本。

## 13. 构建与仓库治理基线

- `web/dist/**` 与 `internal/static/dist/**` 同时受 Git 跟踪，内容当前重复。
- `internal/static/embed.go` 从 `internal/static/dist` 嵌入 SPA。
- Makefile 先在 `web/dist` 构建，再删除并复制到 `internal/static/dist`。
- 仓库当前没有 `.github/**` CI、Dockerfile 或 Compose 文件。
- `.gitignore` 忽略 `bin/`、`*.exe`、`web/node_modules/` 和 `data/`，但没有忽略两份前端 dist。

Task 2 负责消除双产物源和跨平台问题；Task 8 负责 CI、Race、前端 lint/test 和 Secret Scan。

## 14. 已知失败语义索引

| 场景 | 当前结果 | 后续责任 Task |
| --- | --- | --- |
| 配置未知字段 | 静默忽略 | Task 3 |
| 示例固定 Key/首批范围外 Provider | 当前存在 | Task 4 |
| Reload callback 失败 | Reloader/Server 配置指针可能分裂 | Task 5、16 |
| Store 打开失败 | Warn 后继续，静态 Auth fallback | Task 6、45～50 |
| Quota Check 失败 | Fail Open，继续调用上游 | Task 6、51 |
| Usage/Audit 写失败 | 只记日志，请求仍成功 | Task 6、52 |
| SSE JSON/Scanner 错误 | Warn 后 Channel 关闭，Gateway 尝试 `[DONE]` | Task 7、17、22 |
| 客户端 SSE 写失败 | Cancel 后仍走部分成功收尾 | Task 7、17、52 |
| Redis Cache 失败 | 自动切本机 Memory Cache | Task 43、56 |
| Unary Fallback 遇到非 Context 错误 | 继续下一个 Target | Task 37～42 |
| Race 检查 | 当前 CGO 工具链下无法执行 | Task 8、10 |
| 前端构建 | 双产物源，Windows Make 不可复现 | Task 2 |

## 15. Task 1 Exit 判定

- [x] 记录 Go、Node、OS 和 Git 基线。
- [x] 保存 Go Test/Coverage、Vet、Build、Race 和 TypeScript 检查结果。
- [x] 索引数据 API、Legacy Admin、Admin API、SPA 和 Metrics。
- [x] 索引 Config、Reload、Store、Auth、Quota、Provider、SSE 和错误边界。
- [x] 报告只描述当前实现，没有把 v3 目标写成已实现。
- [x] 未修改运行时代码、配置或构建行为。

结论：Task 1 可完成；Task 2 可以进入 Ready。M0 尚未完成，本文列出的失败与风险必须由后续 Task 逐项处理。
