# Go 后端规则

适用于 `cmd/**`、`internal/**`、`config/**` 和其他 Go 运行时代码。

## 分层与接口

- `cmd` 只负责配置加载、依赖组装、生命周期和信号处理。
- HTTP 层只依赖应用服务；应用层依赖 Ports；Provider、Store、Redis、Observer 是 Adapter。
- 接口由消费方定义，保持最小；禁止为了 Mock 建立覆盖整个实现的大接口。
- Canonical/Domain 不导入 Gin、SQL Driver、Redis Client 或厂商 SDK。
- 旧 `provider.ChatRequest` 只允许在迁移适配层继续存在，不新增厂商能力字段。

## Context、并发与生命周期

- 所有阻塞 I/O 和请求链路接收并传播 `context.Context`。
- 不保存或复用请求 Context；不得用 `context.Background()` 切断取消，除非是有独立生命周期的后台任务。
- 每个 goroutine 明确 owner、退出条件和取消；测试取消、超时、关闭与 goroutine 泄漏。
- HTTP Client/Transport 复用并设置分阶段超时；禁止每请求创建 Client，长 SSE 不使用会截断整流的总超时。
- Shutdown 顺序为停止接收、等待 unary、通知/取消 stream、完成结算与审计、关闭依赖。

## 错误与数据

- 包装错误保留操作、稳定分类和 `%w` 原因链；不要暴露秘密或完整上游正文。
- 金额和 Token 使用整数或固定精度，不使用 float；时间统一明确单位。
- 可变 Slice/Map 不跨 Snapshot 或 goroutine 无保护共享；构造完成后再发布。

## 测试与质量

- 表驱动测试覆盖成功、错误、边界、取消；并发状态使用 race test。
- 网络依赖使用 `httptest.Server` 验证真实 Header、Path、Body、SSE 和取消，不只 Mock 方法调用。
- 修改 Go 文件至少执行 gofmt、受影响包测试；按风险执行全量 test、race、vet 和 build。
