# 工程原则与代码通用规范

## 1. 修改前证明根因

每次修改前形成最小根因说明：

```text
现象：用户或系统看到了什么。
触发条件：什么输入、配置、状态或并发时序触发。
根因：哪个职责、状态模型、协议或数据不变量不成立。
影响范围：流式/非流式、单机/多实例、兼容性、安全和数据。
方案：为什么新设计消除根因，而非遮住症状。
验证：哪个复现或测试在修改前失败、修改后通过。
```

禁止用吞错、放宽校验、增加重试/超时、删除协议字段、复制条件分支或修改断言来掩盖根因。

## 2. 正确性与语义保真

- 无法表达的能力必须在执行前得到 `capability_mismatch`，不能静默删除。
- 安全、租户、预算、账本和 Credential 失败不得伪造成功。
- 降级只有在显式策略、能力等价、风险受限、可观测、可恢复且有测试时允许。
- “兼容 OpenAI 协议”不等于“行为与 OpenAI 相同”；厂商忽略参数也属于转换风险，必须检测或记录。
- README、管理端和简历只能描述已通过 Exit Gate 的能力。

## 3. 职责与依赖方向

- HTTP Handler 只做传输层解析、身份上下文和响应编码，不编排 Provider、Quota、Cache 或 Retry。
- Application Service 组织用例；Domain/Canonical 不依赖 Gin、数据库、Redis 或具体厂商 SDK。
- Ingress Codec、Canonical IR、Capability Planner、Provider Codec、Transport、Store 和 Observer 分包并通过稳定接口连接。
- 核心包使用 `context.Context`；不得保存、跨 goroutine 传递或在核心接口暴露 `*gin.Context`。
- 避免跨包共享可变状态；并发读取的配置使用一次构建、一次原子发布的不可变 Snapshot。
- 新依赖必须说明职责、替代方案、维护状态、许可、安全、二进制体积和运行成本。不得为了简历关键词引入未使用的框架。

## 4. 错误模型

- Go 错误使用 `%w` 保留原因链，使用稳定类型或 `errors.Is/As` 分类，不靠字符串匹配。
- 至少区分 invalid_request、authentication、permission、capability_mismatch、budget_exceeded、rate_limited、upstream_overloaded、upstream_timeout、transport、protocol、client_cancelled 和 internal_dependency。
- 错误携带安全的 request ID、阶段、provider code、HTTP status、Retry-After 和 committed 状态；返回客户端前脱敏。
- `context.Canceled`、`context.DeadlineExceeded`、流式 Commit 前后错误必须有不同处理。

## 5. 并发、生命周期与资源

- 启动的 goroutine 必须有 owner、取消路径、退出条件和测试；channel 由发送方关闭。
- 不在持锁状态执行网络、磁盘或可能阻塞的回调。
- HTTP Body、Rows、Transaction、Timer、Ticker、Stream 和 Transport 必须在所有路径释放。
- 请求捕获 Snapshot 后整个生命周期只使用该版本；旧资源在引用归零后关闭。

## 6. 变更边界

- 一个提交解决一个可描述问题，不混入无关重构或格式化。
- 公共 API、Schema、安全、数据或兼容性变化先更新设计/ADR，再编码。
- 不手工修改生成产物；生成源和生成命令必须可复现。
- 发现相邻问题记录为后续项，不擅自扩大授权范围。
