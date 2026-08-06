# 建立旧 Provider 与 Server 端到端夹具

- 日期：2026-08-06 14:22 CST
- 类型：L1
- 关联需求：Task 7
- 影响模块：`internal/provider`、`internal/server`、`docs/baseline`、`docs/AI_Gateway_v3_项目实施任务书.md`

## 根因

旧链路现有测试覆盖了部分 Handler、Cache、Retry 和 Breaker，但 Provider 包没有独立 HTTP 合约测试，多数 Server 测试直接调用内部 Handler，绕过完整 Auth Middleware 和 Gateway 网络边界。后续迁移 Gin、Application Service 或 Canonical IR 时，即使内部 Mock 继续通过，也可能静默改变上游 URL/Header/Body、SSE 顺序、错误分类或 Context 取消传播。

## 解决方案

新增完全离线的 `httptest.Server` Fixture。在 Provider 层覆盖 Unary 编解码、Authorization、429/Retry-After 分类、CRLF/LF SSE、`[DONE]` 和取消；在 Server 层覆盖完整 Auth→Policy→Route→Retry/Fallback→Provider→Response 链路，并断言 virtual model 改写、上游调用次数、429、SSE 首事件/结束标记和客户端取消。

将旧 Fallback 对上游 400 仍继续、SSE 解析错误无法上抛、客户端断开后仍尝试结束标记、旧文本请求表达力不足等现状记录到基线，并映射给 Task 17、19～27、37、40、42、52。Task 7 更新为 Done，Task 8 更新为唯一 Ready 项。

## 行为与兼容性

没有修改运行时代码、公共 API、配置或 Provider 行为。新增测试只固定当前兼容边界；Mock/Fixture 结果不代表任何真实厂商或模型已验证。

## 可观测性

没有修改运行时日志、指标或 Trace。错误夹具验证分类结果不包含 Provider Credential，取消夹具通过上游 Context 信号提供诊断证据。

## 验证

- `[通过]` `go test -count=1 ./internal/provider ./internal/server`。
- `[通过]` `go test -count=1 -cover ./internal/provider ./internal/server`：Provider 41.1%，Server 51.9%。
- `[通过]` WSL `go test -race -count=1 ./internal/provider ./internal/server`。
- `[通过]` `go test -count=1 ./...`。
- `[通过]` `go vet ./...`。
- `[通过]` `go build -o NUL ./cmd/gateway`。
- `[通过]` Credential-shaped literal Secret Scan。
- `[通过]` `git diff --check`。

## 风险与回滚

风险仅限测试对旧链路当前行为的约束。后续契约升级必须同步更新相应断言和基线说明，不能保留已知错误语义。回滚可删除新增夹具和报告并恢复任务状态，不涉及运行时回滚或数据迁移。用户已有的 v2 设计文档删除不属于本任务，不会暂存或提交。
