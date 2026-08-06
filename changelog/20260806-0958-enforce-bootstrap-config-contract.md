# 建立严格 Bootstrap 配置契约

- 日期：2026-08-06 09:58 CST
- 类型：L2
- 关联需求：Task 3
- 影响模块：`config/`、`internal/server/`、`internal/tracing/`、`README.md`、`docs/configuration.md`

## 根因

旧配置入口先对整个文件执行 `os.ExpandEnv`，再使用宽松 `yaml.Unmarshal`。未知字段被
静默丢弃，缺失环境变量被替换为空字符串，错误直到 Provider 构造阶段才暴露；端口、
Duration、比例、策略、重复标识和跨 Provider/Route 引用没有统一校验。默认值还散落在
Server、Provider、Cache 和 Tracing 构造逻辑中，无法区分“字段缺失”和“显式非法零值”。
示例 YAML 的 `read_timeout`/`write_timeout` 没有对应 Go 字段且从未生效。

## 解决方案

- 将加载固定为读取、环境变量检查、`KnownFields` 单文档解码、集中默认值、完整校验五个
  阶段，任一阶段失败都不返回部分配置。
- 新增 `ConfigError` 和 `read`、`environment`、`parse`、`validation` 四类稳定错误，
  保留底层错误链且只报告安全字段路径。
- 在解码前预置普通字段默认值，使显式 `0`、`0s` 和空字符串能够进入校验而不是被覆盖；
  Provider 列表通过 YAML Node 记录 timeout 是否出现，区分缺失、零值和 null。
- 校验字段范围、枚举、URL、重复 Provider/Route/Model/Key、Route Match 冲突、Target
  Provider/Model、Key Route Allowlist、Semantic Rule 完整性以及 Cache/Tracing/Filter
  契约。
- 增加 Server `read_timeout`/`write_timeout` 字段并让 `http.Server` 实际消费。
- 新增表驱动测试、错误 Golden、最小配置默认值测试、当前示例测试和配置契约文档。

Task 3 状态更新为 Done，Task 4 更新为唯一 Ready 项。

## 行为与兼容性

这是有意收紧的启动/Reload 配置契约：未知字段、无效值、重复标识和悬空引用不再被
忽略或推迟到构造阶段。旧配置如果依赖未知字段、Target 指向 Provider 未声明模型、缺失
环境变量或用显式零值触发默认行为，升级前必须修复。合法最小配置继续获得确定默认值。

当前 Provider Type 仍保留迁移期 `openai`/`claude`，没有提前实施 Task 4 的三厂商独立
Schema。没有 API、数据库或数据迁移变化。

## 可观测性

启动和 Reload 现有错误日志现在包含稳定阶段与安全字段路径，例如 `config environment
error at environment` 或 `config validation error at routes[0].strategy`。错误不会打印
API Key、环境变量值或完整配置。本次没有新增指标或 Trace；Runtime Snapshot 发布/拒绝
的结构化事件属于 Task 5。

## 验证

- `[通过]` `go test -count=1 -cover ./config`，覆盖率 70.1%。
- `[通过]` `go test -count=1 ./...`。
- `[通过]` `go vet ./...`。
- `[通过]` `go run ./cmd/build -target backend`，生成 Windows 网关二进制。
- `[通过]` 当前 `config/gateway.yaml` 在提供三个测试环境变量时通过严格加载，并验证
  `read_timeout=30s`、`write_timeout=120s`。
- `[通过]` 缺失三个 Provider 环境变量的启动负向 Smoke 在 Provider 构造前退出，输出
  稳定 `environment` 分类和排序变量名，不包含值。第一次 Smoke 脚本写错了预期字典序，
  实现输出本身正确；修正测试断言后通过。
- `[通过]` `git diff --check`、错误 Golden 和新增行秘密模式扫描。
- `[未执行]` `go test -race ./...`；Task 1 已记录当前 Windows 环境 `CGO_ENABLED=0`
  且无 gcc，本 Task 不修改共享配置发布或 Reload 并发，Race 门禁仍由 Task 8 处理。

## 风险与回滚

最主要风险是旧配置在升级时被严格拒绝；错误字段路径和迁移文档用于显式修复，禁止通过
关闭校验恢复旧行为。回滚需同时恢复宽松 Loader、构造期默认值和固定 HTTP Timeout，
不涉及数据回滚。用户此前删除的 v2 文档不纳入本任务提交。
