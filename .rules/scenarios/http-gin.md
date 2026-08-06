# Gin、HTTP 与 SSE 边界规则

## 架构边界

- 目标架构使用两个独立 `gin.Engine` 和两个监听端口：数据面与控制面不共享完整 Middleware 链。
- 使用 `gin.New()` 显式安装 Recovery、日志和安全 Middleware；不依赖 `gin.Default()` 隐式行为。
- `*gin.Context` 只存在于 Edge。进入应用层前转换为普通 DTO、Identity 和 `context.Context`。
- Gin 是入口生产力工具，不是领域框架；核心服务必须可在无 HTTP 的单元测试中调用。

## 数据面

推荐顺序：`RequestID -> Trace -> Recovery -> SecurityHeaders -> Auth -> Tenant/Project -> Admission -> RateLimit -> Handler`。

- Body Limit、严格 JSON 解码、Content-Type、协议版本和错误 Envelope 必须一致。
- 需要解析 Prompt/Tools 的策略属于 GenerationService，不塞进 Middleware，也不重复读取 Body。
- SSE Handler 直接使用 `http.ResponseWriter`/`http.Flusher`；先设置 Header，再写状态与事件。
- 压缩、响应缓冲和统一 Response Recorder 不得破坏 Flush；流式路径必须有专门测试。
- 客户端取消从 Request Context 传播到 Planner、Provider、解析器、Quota 和 Audit。
- 写出首个合法协议事件后标记 committed；此后禁止 Retry/Fallback 或改写 HTTP Status。

## 控制面

推荐顺序：`RequestID -> Trace -> Recovery -> OIDC/Session -> CSRF -> RBAC -> DTO Validation -> Handler -> Audit`。

- 使用显式 Binding/Validation 和统一错误映射，避免 MustBind 提前写响应。
- OpenAPI/Schema 是控制面契约来源；Handler 不返回数据库实体或秘密字段。
- 写操作包含审计主体、对象、变更摘要和结果；乐观锁或 revision 防止静默覆盖。

## Server 生命周期

- 分别配置 Header、Idle、Read 和 Shutdown 策略；长流不设置固定 WriteTimeout 截断有效 SSE。
- Liveness 只说明进程存活；Readiness 反映 Snapshot、Store、必要 Credential 和迁移状态。
- 测试 panic recovery、超大 Body、无效 JSON、慢客户端、断流、双端口隔离和 graceful shutdown。
