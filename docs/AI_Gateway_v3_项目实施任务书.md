# AI Gateway v3 项目实施任务书

> 状态：Execution Baseline
>
> 日期：2026-08-06
>
> 适用代码基线：`master@9489636`
>
> 关联设计：`docs/AI_Gateway_v3_企业级重构设计文档.md`
>
> 执行原则：同一时间只实施一个 Task；当前 Task 验收、记录并提交后，才能开始下一个 Task。

## 1. 文档目的

v3 设计定义“最终要成为什么”，本任务书定义“从当前代码如何逐步到达”。它把 M0～M6 拆成可独立审查、验证和提交的 Task，防止一次重构同时修改框架、协议、路由、存储和前端，导致根因、回归和责任边界无法判断。

任务编号和顺序是执行契约，不是建议清单。除非先更新本任务书并说明依赖变化，否则不得跳过 Task、合并多个 Task 或提前宣传后续能力。

## 2. 当前基线证据

2026-08-06 在 Windows/amd64、Go 1.26.5 上完成只读基线检查：

- `go test ./...`、`go vet ./...`、`go build ./cmd/gateway` 通过。
- `config`、`internal/store`、`cmd/gateway`、`internal/metrics`、`internal/observer` 等关键包没有测试。
- `web/dist/**` 与 `internal/static/dist/**` 同时被 Git 跟踪，前端产物存在两个事实源。
- Makefile 的复制/删除命令依赖 Unix Shell，Windows 无法直接复现完整构建。
- 前端只有 `build` 脚本，没有 lint 和 test 门禁；仓库没有 CI 工作流、Dockerfile 或 Compose 验收环境。
- 配置加载不拒绝未知字段、不集中应用默认值和交叉校验；示例配置含固定演示 Key，并仍以通用 OpenAI-compatible 类型描述上游。
- 旧 `Server`、`provider.ChatRequest` 和 Provider 实现仍是迁移起点，本任务书不得把 v3 目标描述为当前已实现。

用户此前删除的 v2 文档不是本计划资产，执行任务时不得恢复或顺带提交。

## 3. Task 执行协议

### 3.1 状态

- `Ready`：所有前置 Task 已完成，可成为唯一当前任务。
- `Pending`：已规划，但依赖尚未完成。
- `Done`：交付物、测试、changelog 和原子提交全部完成。
- `Blocked`：同一阻塞连续确认后仍无法在授权范围内解决；必须写明证据和所需输入。

本文初始只有 Task 1 为 `Ready`。Task 的实时 `In Progress` 状态在执行计划中维护；完成时在同一实现提交中把本文状态更新为 `Done`，并将下一项改为 `Ready`。

### 3.2 开始一个 Task

1. 检查工作树，保护用户修改。
2. 阅读 `AGENTS.md`、核心规则、Task 命中的场景规则、关联设计和现有代码/测试。
3. 写出该 Task 的现象、触发条件、根因、影响范围、方案和验证。
4. 确认前置 Task 均为 `Done`，且当前没有另一个实施中的 Task。
5. 只把本 Task 的交付物加入本轮范围。

### 3.3 完成一个 Task

每个 Task 默认对应一个原子提交，并同时满足：

- 目标不变量已实现，没有临时旁路或静默降级。
- 成功、失败、边界、取消和相关并发路径有测试。
- 运行时决策有安全的日志、指标或 Trace；纯文档任务标记不适用。
- API、配置、Schema、迁移、README 和示例按实际行为同步。
- 执行 Task 指定验证和所有受影响的基础检查。
- 新建 changelog，本文状态同步为 `Done`，下一 Task 改为 `Ready`。
- 显式暂存本 Task 文件并提交；默认不 push。

### 3.4 发现计划外问题

- 不把相邻问题偷偷塞进当前 Task。
- 若是当前 Task 完成所必需，先在本文插入 `Task N.R1` 修复项，写清根因、依赖和验收，再实施。
- Exit Gate 失败时，Gate 保持未完成；重新打开责任 Task 或插入 Remediation Task，修复后重新执行 Gate。
- 任何新增 Task 都不得降低既有 Exit Gate。

## 4. 总体阶段与求职切线

| 阶段 | Task | 结果 | 主线状态 |
| --- | --- | --- | --- |
| M0 可信基线 | 1～10 | 构建、配置、安全、测试和 CI 可信 | Task 1 Ready |
| M1 Gin 与应用边界 | 11～18 | 双平面 Gin，核心与框架解耦 | Pending |
| M2 Canonical 与双 Ingress | 19～27 | Chat/Responses 进入同一语义模型 | Pending |
| M3 国内三厂商 | 28～36 | 方舟、DeepSeek、Qwen 可合约与真实验证 | Pending |
| M4 路由与韧性 | 37～44 | 能力等价路由、Retry/Fallback/Breaker 正确 | Pending |
| M5 企业控制面与账本 | 45～55 | 多租户、Credential、配置发布、预算账本 | Pending |
| M6 多实例与发布 | 56～62 | 两实例、真实调用方、性能与故障报告 | Pending |

秋招第一交付线是 Task 36：完成前只描述“正在重构”；完成后才具备“Gin + Canonical IR + 国内三厂商 Adapter”的完整项目叙事。最终企业级交付线是 Task 62。

---

## 5. M0：可信基线

### Task 1：建立可复现的当前行为基线

- **状态：** Ready
- **依赖：** 无
- **目标：** 让后续变更能判断是旧问题还是新回归。
- **交付：** `docs/baseline/` 基线报告；记录 Go/Node/OS、启动命令、公开路由、配置来源、当前包覆盖和已知失败语义；保存不含秘密的测试命令与结果。
- **验收：** 新机器可按报告执行 Go test/vet/build；当前公开 API、管理 API、SSE、Store、Reload 的行为边界都有入口索引；报告只陈述当前事实。
- **验证：** `go test ./...`、`go vet ./...`、`go build ./cmd/gateway`、工作树差异检查。
- **不包含：** 修复构建、改配置、引入 Gin 或改变运行行为。

### Task 2：统一前端构建产物与跨平台构建入口

- **状态：** Pending
- **依赖：** Task 1
- **目标：** 消除 `web/dist` 与嵌入目录双事实源和 Unix-only 构建。
- **交付：** 唯一前端生成源、明确的 embed 输入、Windows/Unix 可复现构建脚本、生成产物 Git 策略、Makefile/README 同步。
- **验收：** 全新检出可一条命令构建；连续构建结果一致且不会产生非预期工作树修改；Go embed 只引用唯一产物位置。
- **验证：** 前端 clean install/build、Go build、两次构建哈希/工作树检查。
- **不包含：** 修改页面功能或视觉样式。

### Task 3：建立严格配置加载、默认值与校验契约

- **状态：** Pending
- **依赖：** Task 2
- **目标：** 无效配置在启动/发布边界明确失败，而不是以零值或未知字段继续运行。
- **交付：** `KnownFields` 严格解析、集中默认值、字段范围和交叉引用校验、缺失环境变量检测、稳定错误分类；补齐当前 YAML 与 Go Struct 不一致字段。
- **验收：** 未知字段、非法 duration/port/ratio、重复 ID、悬空 provider/route、缺失 env、无效 strategy 全部可定位失败；合法最小配置产生确定默认值。
- **验证：** `config` 表驱动测试、Golden 错误、`go test ./...`。
- **不包含：** 控制面数据库配置和完整 Runtime Snapshot。

### Task 4：清理秘密示例并收敛 Bootstrap Provider 配置

- **状态：** Pending
- **依赖：** Task 3
- **目标：** 示例配置不能看似携带可用 Key，也不能把未验证厂商写成已支持。
- **交付：** 无效占位 Key、环境变量 Secret reference、Ark/DeepSeek/Qwen 独立 provider kind 的 Bootstrap Schema；删除首批范围外的默认路由声明。
- **验收：** 仓库 Secret Scan 无 Credential；示例无真实调用权限；未实现 Adapter 只能标记 disabled/unverified，启动错误可操作。
- **验证：** 配置测试、Secret Scan、README 配置示例检查。
- **不包含：** 三家 Adapter 实现和 Credential 加密存储。

### Task 5：固定配置 Reload 的正确性边界

- **状态：** Pending
- **依赖：** Task 3、Task 4
- **目标：** 禁止部分配置已生效、其他组件仍使用旧配置的伪成功 Reload。
- **交付：** 当前阶段可原子替换的配置快照、dynamic/restart-required 分类、Reload 接受/拒绝日志和回归测试。
- **验收：** 构建或校验失败时旧配置完整保留；restart-required 变更明确拒绝；并发请求不观察到混合版本。
- **验证：** Reload 成功/失败/并发测试、`go test -race ./config ./internal/server`。
- **不包含：** Transport/Store 等全部依赖的引用计数回收，留给 Task 16。

### Task 6：为 Store、Migration、Key 和 Quota 建立保护测试

- **状态：** Pending
- **依赖：** Task 1
- **目标：** 固定当前持久化语义，暴露迁移、并发额度和错误处理缺口。
- **交付：** 临时 SQLite Repository 测试、Migration 幂等测试、Key 生命周期、Quota 并发与 Store 故障测试；已知错误以失败证据进入后续 Ledger Task。
- **验收：** 测试不共享开发数据库；重复迁移稳定；并发检查能证明当前是否穿透；Store 错误不被测试夹具吞掉。
- **验证：** `go test -race ./internal/store ./internal/middleware`。
- **不包含：** PostgreSQL、租户模型和新 Ledger 状态机。

### Task 7：为旧 Provider 与 Server 建立端到端回归夹具

- **状态：** Pending
- **依赖：** Task 1
- **目标：** 在迁移 Gin/Canonical 前锁定当前 Chat、SSE、错误和取消行为。
- **交付：** `httptest.Server` 上游、Unary/SSE Fixture、Auth/Route/Retry/Fallback/取消测试、脱敏测试数据。
- **验收：** 不访问真实厂商；能断言 URL/Header/Body、首事件、结束标记、客户端取消和上游错误分类；Fixture 不含秘密。
- **验证：** `go test -race ./internal/provider ./internal/server`。
- **不包含：** 为旧抽象添加 Tools、Reasoning 或多模态字段。

### Task 8：建立后端与前端质量门禁

- **状态：** Pending
- **依赖：** Task 2、Task 3、Task 6、Task 7
- **目标：** 所有后续 Task 有自动、可重复的最低质量门槛。
- **交付：** CI 工作流；Go format/test/race/vet/build、静态分析、Secret Scan；前端 lint/test/build 脚本和最小测试框架；依赖缓存不影响正确性。
- **验收：** 新提交自动执行；任一故意制造的格式、测试、Secret 或构建错误会使 CI 失败；任务产物不污染 Git。
- **验证：** 本地等价命令和 CI 首次成功记录。
- **不包含：** Adapter Conformance、Migration Integration 和 Docker Smoke，后续增量加入。

### Task 9：校准 README、运行说明与事实等级

- **状态：** Pending
- **依赖：** Task 2～Task 8
- **目标：** 对外说明与当前可验证能力一致。
- **交付：** README 的 Implemented/Planned/Unverified 能力表、跨平台启动、配置/Secret、已知限制和 v3 路线链接。
- **验收：** 不出现“全厂商兼容/生产级”等无证据表述；命令在新环境可执行；客户端协议兼容与上游厂商适配分开描述。
- **验证：** 文档命令 Smoke、链接和示例检查。
- **不包含：** 提前写入 Task 36/62 才能使用的简历表述。

### Task 10：执行 M0 Exit Gate

- **状态：** Pending
- **依赖：** Task 1～Task 9
- **目标：** 证明仓库已经具备安全开始架构迁移的可信地基。
- **交付：** `docs/gates/M0.md`，记录环境、命令、结果、残余风险和下一阶段许可。
- **验收：** 全新检出可构建启动；CI 全绿；构建不污染工作树；Config/Store/Provider/Server/Reload 有保护测试；无固定演示秘密。
- **验证：** M0 全量 CI 命令、race、Secret Scan、启动/readiness Smoke。
- **不包含：** M1 功能；Gate 失败不得通过修改报告改成成功。

---

## 6. M1：Gin 入口与应用边界

### Task 11：确定 Gin 双平面与应用边界 ADR

- **状态：** Pending
- **依赖：** Task 10
- **目标：** 在编码前固定 Engine、端口、Middleware、DTO、SSE 和依赖方向。
- **交付：** ADR：双 `gin.Engine`/双 `http.Server`、`gin.Context` 边界、错误 Envelope、迁移与回滚方案、依赖选择依据。
- **验收：** 数据面/控制面路由与 Middleware 顺序明确；核心包禁止依赖 Gin；旧 API 兼容策略可测试。
- **验证：** ADR 与 v3/.rules 一致性检查。
- **不包含：** 引入 Gin 或修改运行代码。

### Task 12：建立统一请求上下文与错误模型

- **状态：** Pending
- **依赖：** Task 11
- **目标：** Handler 与核心通过稳定类型交换身份、请求信息和错误，不传递框架对象。
- **交付：** Request Context/Identity、分类错误、错误 Envelope 和安全映射；request ID/trace ID 传播测试。
- **验收：** `context.Canceled`、deadline、输入、权限、上游和内部错误可机器区分；客户端不看到堆栈或秘密。
- **验证：** 错误映射表驱动测试、日志脱敏测试。
- **不包含：** Provider 新协议和 Retry 策略重写。

### Task 13：从旧 Server 提取 GenerationService

- **状态：** Pending
- **依赖：** Task 12
- **目标：** 把 Chat 用例编排从 HTTP Handler 移到框架无关应用服务。
- **交付：** `internal/app/generation`、最小 Ports、旧 Provider/Store/Router 适配；旧 net/http Handler 只做 DTO 转换。
- **验收：** GenerationService 可无 HTTP 单测；Handler 不直接访问具体 Provider/Store；Task 7 回归保持一致。
- **验证：** 应用服务单测、旧 Server 回归、`go test -race ./internal/app/... ./internal/server`。
- **不包含：** Canonical IR 和新厂商能力。

### Task 14：实现 Gin 数据面兼容入口

- **状态：** Pending
- **依赖：** Task 12、Task 13
- **目标：** 用 Gin 承载现有 `/v1/chat/completions`，核心行为保持不变。
- **交付：** `gin.New()` 数据面、显式 Middleware、Body Limit/JSON/Content-Type、错误 Envelope、到 GenerationService 的 Adapter。
- **验收：** Unary 与旧客户端兼容；Gin Context 不越过 Edge；超大 Body、无效 JSON、panic 和取消有确定响应。
- **验证：** Gin Handler/Integration 测试、Task 7 回归对照。
- **不包含：** `/v1/responses` 和 Canonical 新字段。

### Task 15：实现 Gin 控制面与双 Server 生命周期

- **状态：** Pending
- **依赖：** Task 14
- **目标：** 数据面、控制面在网络、超时和鉴权上真正隔离。
- **交付：** 控制面 Engine、双监听配置、health/live/ready/metrics/admin SPA 路由、独立 Middleware 与生命周期组装。
- **验收：** 管理身份不能访问数据面特权，数据 Key 不能访问控制面；任一 Server 启动失败会有序回滚；端口冲突可诊断。
- **验证：** 双端口集成测试、权限反向测试、启动失败和 Shutdown 测试。
- **不包含：** OpenAPI-first 新管理接口和 OIDC。

### Task 16：实现 Runtime Snapshot v1

- **状态：** Pending
- **依赖：** Task 5、Task 13、Task 15
- **目标：** 请求生命周期只使用一个完整运行时依赖版本。
- **交付：** 不可变 Snapshot、完整构建/校验、原子发布、revision、旧资源引用与延迟关闭、发布诊断。
- **验收：** Router/Provider/Cache/Policy 不混用版本；构建失败保留旧 Snapshot；并发请求和资源回收无 Race/泄漏。
- **验证：** 并发切换、失败回滚、资源回收、`go test -race`。
- **不包含：** 数据库草稿/发布/Outbox，留给 Task 50。

### Task 17：建立 SSE Commit、取消与优雅关闭语义

- **状态：** Pending
- **依赖：** Task 14～Task 16
- **目标：** Gin 迁移不破坏长流连接正确性。
- **交付：** Flusher/Commit Tracker、客户端取消传播、流注册表、Shutdown drain/cancel 顺序、首事件和写失败诊断。
- **验收：** Commit 前可返回标准 HTTP 错误；Commit 后不改状态、不伪造 `[DONE]`；取消关闭上游并触发确定结算钩子。
- **验证：** 慢客户端、断流、异常 EOF、取消、Shutdown 与 goroutine leak 测试。
- **不包含：** Canonical typed events 和跨厂商 Fallback。

### Task 18：执行 M1 Exit Gate

- **状态：** Pending
- **依赖：** Task 11～Task 17
- **目标：** 证明框架迁移完成但核心未被 Gin 绑定。
- **交付：** `docs/gates/M1.md` 和架构依赖检查。
- **验收：** 双 Engine/双监听可运行；Handler 不编排具体基础设施；核心包不导入 Gin；Chat 回归、取消和 graceful shutdown 全部通过。
- **验证：** 全量 Go/前端 CI、race、双 Server Smoke、依赖扫描。
- **不包含：** M2 协议能力。

---

## 7. M2：Canonical IR 与双 Ingress

### Task 19：确定 Canonical、Capability 与 Translation ADR

- **状态：** Pending
- **依赖：** Task 18
- **目标：** 固定公共语义、不变量、扩展边界和版本策略。
- **交付：** ADR：Item/Block/Event、Provider State、Capability、Translation Report、未知字段、版本与兼容策略。
- **验收：** IR 既不绑定某一家 DTO，也不退化成文本最低公共子集；exact/normalized/lossy/unsupported 判定明确。
- **验证：** 用方舟、DeepSeek、Qwen 官方样例做纸面映射审查。
- **不包含：** Go 类型和 Adapter。

### Task 20：实现 Canonical Request/Response 与不变量

- **状态：** Pending
- **依赖：** Task 19
- **目标：** 建立能表达文本、工具、推理、结构化和多模态的领域模型。
- **交付：** typed Item/Content Block、Tool Definition/Call/Result、Reasoning、Usage、Finish Reason、Provider State、构造器与校验。
- **验收：** ID/顺序/call_id 引用稳定；非法角色、孤立 Tool Result、冲突输出约束和无效内容在构造阶段失败。
- **验证：** 表驱动不变量测试、JSON round-trip、Fuzz Smoke。
- **不包含：** HTTP DTO 或厂商 JSON tag。

### Task 21：实现 Capability 与 Translation Report

- **状态：** Pending
- **依赖：** Task 20
- **目标：** 在执行前知道请求需要什么、转换会损失什么。
- **交付：** Required Capabilities 推导、Support/Evidence 状态、Translation Entry/Severity、稳定 capability mismatch 错误。
- **验收：** Tools、Reasoning、Structured Output、Multimodal、Streaming、Usage、State 分别判断；unsupported 不能执行，lossy 需要显式策略。
- **验证：** 能力推导和转换策略矩阵测试。
- **不包含：** 实际路由算法和厂商能力数据。

### Task 22：实现 Canonical 流事件与通用 SSE Parser

- **状态：** Pending
- **依赖：** Task 20、Task 21
- **目标：** 流不再用无类型字符串拼接。
- **交付：** typed events、sequence/index/item/call ID、流状态机、SSE frame parser、大小限制和错误类型。
- **验收：** 支持跨 Buffer、CRLF、多 data 行、Unicode、heartbeat、流内 Error、异常 EOF；非法状态转换失败。
- **验证：** Golden Replay、Fuzz、异常流和内存上限测试。
- **不包含：** 各厂商事件 Decoder。

### Task 23：实现 OpenAI Chat Completions Ingress Codec

- **状态：** Pending
- **依赖：** Task 20～Task 22
- **目标：** 将客户端 Chat 请求/响应完整映射到 Canonical。
- **交付：** 请求 Decoder、响应/Chunk Encoder、Tool、JSON/Schema 意图、Reasoning 扩展、多模态内容、Usage 与错误映射。
- **验收：** 未知/不支持字段按策略处理；Tool delta 和 finish reason 保序；严格解码错误可机器识别。
- **验证：** 请求/响应/SSE Golden、Round-trip 可表达子集测试。
- **不包含：** OpenAI 上游 Provider。

### Task 24：实现 OpenAI Responses Ingress Codec

- **状态：** Pending
- **依赖：** Task 20～Task 22
- **目标：** 让 Responses 客户端与 Chat 客户端进入同一 Canonical 流程。
- **交付：** `/v1/responses` Decoder、typed event Encoder、Item/Tool/Reasoning/State/Structured Output/Usage 映射。
- **验收：** typed item 不降格字符串；`previous_response_id` 等状态要求显式表示；不支持的异步/Realtime 参数在上游前拒绝。
- **验证：** Unary/Stream Golden、未知事件、状态引用和错误映射测试。
- **不包含：** 方舟/Qwen Responses Egress。

### Task 25：将 GenerationService 迁移到 Canonical Execution Port

- **状态：** Pending
- **依赖：** Task 13、Task 20～Task 24
- **目标：** Chat 与 Responses 共享应用用例，旧 Provider 隔离在迁移 Adapter 后。
- **交付：** Canonical Generate/Stream 用例、Execution Port、Ingress Registry、旧 Chat 兼容 Adapter、统一审计上下文。
- **验收：** 双入口走同一服务；应用层不导入 Ingress/Provider DTO；旧 Chat 回归仍通过。
- **验证：** 应用服务单测、双入口集成、取消与错误传播测试。
- **不包含：** 三家正式 Adapter 和多目标 Planner。

### Task 26：实现执行前能力拒绝与转换策略

- **状态：** Pending
- **依赖：** Task 21、Task 25
- **目标：** 不兼容请求永远不触达上游。
- **交付：** preflight capability gate、Translation Policy、错误 Envelope、拒绝日志/指标/Trace 属性。
- **验收：** 删除 Tool/Reasoning/Schema 等静默降级路径；unsupported 请求上游调用次数为 0；lossy 决策可审计。
- **验证：** 负向能力测试、上游零调用断言、日志/指标字段测试。
- **不包含：** 多候选选择和 Fallback。

### Task 27：执行 M2 Exit Gate

- **状态：** Pending
- **依赖：** Task 19～Task 26
- **目标：** 证明协议入口和内部语义层可独立于厂商演进。
- **交付：** `docs/gates/M2.md`、Canonical/Ingress Golden 清单和覆盖证据。
- **验收：** 双入口同服务；typed event 保真；能力不支持在上游前拒绝；核心无 Gin/厂商 DTO 依赖。
- **验证：** 全量 CI、Canonical/Ingress Fuzz、race、Unary/SSE 集成。
- **不包含：** M3 厂商真实性声明。

---

## 8. M3：火山方舟、DeepSeek、Qwen Adapter

### Task 28：建立 Provider Adapter Contract、Transport 与 Conformance Harness

- **状态：** Pending
- **依赖：** Task 27
- **目标：** 三家共享测试契约和传输能力，但不共享厂商语义 Codec。
- **交付：** Adapter/Codec/Transport 接口、受控 Endpoint/Auth、连接池/超时/TLS/Redirect 策略、错误分类、Conformance Suite。
- **验收：** Codec 无网络；Transport 不理解 Canonical；每个 Adapter 必须通过相同 Text/Stream/Error/Usage 基础合约。
- **验证：** `httptest.Server` 的 URL/Header/Auth/取消/超时/Redirect 测试。
- **不包含：** 具体厂商字段映射。

### Task 29：实现火山方舟 Responses Adapter

- **状态：** Pending
- **依赖：** Task 28
- **目标：** 原生承载方舟 typed Responses，而不是只转 Chat 文本。
- **交付：** Request Encoder、Unary/Stream Decoder、typed output item、function call/result、`call_id`、`previous_response_id`、thinking、Usage、错误 Fixture。
- **验收：** State/Tool/Thinking 不降格；未知完成相关事件返回 protocol error；官方样例全部进入脱敏 Golden。
- **验证：** 双向 Golden、SSE Replay、Conformance、异常事件测试。
- **不包含：** 内置工具的实际业务执行。

### Task 30：完成方舟 Chat Dialect、能力证据与真实 Smoke

- **状态：** Pending
- **依赖：** Task 29
- **目标：** 将方舟 Chat 与 Responses 作为两个明确版本验收。
- **交付：** Chat Codec、Ark endpoint/region/model Capability Evidence、`ARK_API_KEY` opt-in Smoke、脱敏结果报告。
- **验收：** Chat 不复用未验证 Responses 参数；每项 verified 能力都有模型、地域、Endpoint、日期和 Adapter revision。
- **验证：** Chat Golden/Stream/Error、真实 Text/Stream/Tool/Thinking Smoke。
- **不包含：** 没有账户权限的模型标记 verified。

### Task 31：实现 DeepSeek Chat、SSE 与 JSON Output Adapter

- **状态：** Pending
- **依赖：** Task 28
- **目标：** 正确表达 DeepSeek 的 Chat、推理分离和 JSON Object 语义。
- **交付：** Request/Response/SSE Codec、`reasoning_content`、content、Usage、JSON Object 约束、错误映射。
- **验收：** reasoning 与 final content 顺序明确；JSON Object 不冒充 JSON Schema；空内容风险有确定处理和诊断。
- **验证：** Golden、SSE Replay、JSON/空内容/异常 EOF 测试。
- **不包含：** thinking + tool 多轮回传，留给 Task 32。

### Task 32：完成 DeepSeek Thinking + Tool 状态与真实 Smoke

- **状态：** Pending
- **依赖：** Task 31
- **目标：** 解决 DeepSeek 最有区分度的 reasoning 状态回传要求。
- **交付：** assistant reasoning state 持久映射、tool call/result 多轮编码、缺失 reasoning 的回归测试、stable/beta strict capability 分离、`DEEPSEEK_API_KEY` Smoke。
- **验收：** Tool 后续请求完整回传 `reasoning_content`；stable 与 beta endpoint 不混用；无 Key 状态为 unverified。
- **验证：** 多轮 Tool Golden、故意丢失状态的负向测试、真实 Text/Stream/Reasoning/Tool Smoke。
- **不包含：** 把 Beta strict 默认加入生产路由。

### Task 33：实现 Qwen Chat、Thinking 与多模态 Dialect

- **状态：** Pending
- **依赖：** Task 28
- **目标：** 正确处理 Qwen Chat 的模型相关能力和流式 Usage。
- **交付：** Chat Codec、`enable_thinking`、`reasoning_content`、`stream_options.include_usage`、Tool、JSON Object、多模态 content block、仅流式限制。
- **验收：** 不假定所有 Qwen 模型能力相同；Usage 不从普通 chunk 伪造；不支持组合在执行前拒绝。
- **验证：** 模型矩阵 Golden、SSE、Tool/JSON/Multimodal 和缺 Usage 测试。
- **不包含：** Responses typed state，留给 Task 34。

### Task 34：实现 Qwen Responses Adapter

- **状态：** Pending
- **依赖：** Task 33
- **目标：** 原生表达百炼 Responses 的 typed output、状态和内置工具事件。
- **交付：** Responses Encoder/Decoder、`previous_response_id`、`reasoning.effort`、typed events、built-in tool item、Workspace/region Endpoint 配置。
- **验收：** 未在官方文档列出的兼容参数不盲目透传；旧 URL 与新 Workspace URL 有显式版本/迁移策略。
- **验证：** Unary/SSE Golden、State/Tool/Unknown parameter 测试。
- **不包含：** 网关自己执行百炼内置工具。

### Task 35：建立三厂商 Capability Evidence 与统一真实验证入口

- **状态：** Pending
- **依赖：** Task 30、Task 32、Task 34
- **目标：** 能力声明由证据驱动，而不是由厂商名称或 Mock 决定。
- **交付：** 版本化能力清单、documented/experimental/verified/unverified/unsupported 状态、统一 opt-in Smoke CLI/报告、Secret 脱敏。
- **验收：** 能力键包含 provider/endpoint/region/model/protocol/adapter revision；没有 Key 不失败 CI，但绝不变成 verified。
- **验证：** Offline Conformance 必跑；持有的三家 Credential 分别运行 Smoke；报告 Secret Scan。
- **不包含：** 多目标自动路由。

### Task 36：执行 M3 Exit Gate

- **状态：** Pending
- **依赖：** Task 28～Task 35
- **目标：** 形成秋招可演示的国内三厂商协议工程闭环。
- **交付：** `docs/gates/M3.md`、能力矩阵、脱敏真实 Smoke 报告、演示脚本与当前简历候选表述。
- **验收：** 三家独立 Adapter/Fixture；Tool/Reasoning/Usage/Provider State/Stream Error 不静默丢失；所有声称 verified 的组合有真实证据。
- **验证：** 全量 CI、Conformance、Fuzz、race、三家可用 Credential Smoke、端到端 Chat/Responses Demo。
- **不包含：** M4 多目标路由和企业账本能力表述。

---

## 9. M4：能力路由与韧性状态机

### Task 37：确定 Planner、Retry、Fallback 与 Commit ADR

- **状态：** Pending
- **依赖：** Task 36
- **目标：** 固定候选筛选、错误分类、重试边界、流式 Commit 和成本约束。
- **交付：** ADR：ExecutionPlan、不变量、分阶段错误、幂等、Retry-After、Fallback、Hedging 和 Breaker 隔离键。
- **验收：** Commit 前后行为、能力等价、预算调整和取消语义均可判定。
- **验证：** 用 429、5xx、timeout、tool mismatch、stream abort 场景做状态机审查。
- **不包含：** 运行代码。

### Task 38：实现 Virtual Model、Capability Planner 与 ExecutionPlan

- **状态：** Pending
- **依赖：** Task 21、Task 35、Task 37
- **目标：** 请求只进入满足协议与模型能力的候选集。
- **交付：** Virtual Model、Target Evidence、不可变 ExecutionPlan、候选接受/拒绝原因、Snapshot revision。
- **验收：** 筛选顺序为能力、权限、地域/合规、预算、健康、成本/延迟/质量；不兼容目标永不进入执行链。
- **验证：** 决策表测试、确定性/并发读取测试、负向能力矩阵。
- **不包含：** Retry/Fallback 执行。

### Task 39：实现策略约束与 Route Dry Run

- **状态：** Pending
- **依赖：** Task 38
- **目标：** 路由选择可解释、可预演且不调用上游。
- **交付：** 租户/项目/Key、region、budget、max cost、健康和调度策略；Dry Run 应用接口与诊断输出。
- **验收：** Dry Run 与真实 Planner 使用同一逻辑；输出每个候选的能力证据和拒绝原因；不暴露 Credential。
- **验证：** Policy/Dry Run 一致性和越权反向测试。
- **不包含：** 完整企业租户持久化，先使用明确 Port/Fixture。

### Task 40：实现统一错误分类与分阶段 Retry

- **状态：** Pending
- **依赖：** Task 28、Task 37～Task 39
- **目标：** 只对安全、可重试且未取消的失败重试。
- **交付：** 阶段化错误、attempt budget、Deadline/Retry-After、jitter、POST 幂等证明、重试日志/指标/Trace。
- **验收：** invalid/capability/auth/cancel 不重试；可能已被接收的非幂等 POST 不盲重试；总预算不会被单次 timeout 绕过。
- **验证：** 429/5xx/connect/write/read timeout、取消、Retry-After 和次数测试。
- **不包含：** 跨目标 Fallback。

### Task 41：实现 Target 级 Circuit Breaker

- **状态：** Pending
- **依赖：** Task 38、Task 40
- **目标：** 故障隔离到 endpoint/model/region，而不是误伤整个厂商。
- **交付：** target key、失败计权、half-open 并发、状态事件、管理只读视图。
- **验收：** 客户端 4xx/能力/取消不计故障；状态恢复可观测；高并发下状态转换无 Race。
- **验证：** 状态机、half-open 并发、恢复、错误分类和 race 测试。
- **不包含：** Redis 分布式 Breaker；本地保护为主。

### Task 42：实现能力等价 Fallback 与流式 Commit Barrier

- **状态：** Pending
- **依赖：** Task 38～Task 41
- **目标：** 只在客户端尚未看到响应且目标仍满足语义/预算时切换。
- **交付：** Fallback Executor、重新 Capability/Policy/Budget 检查、Reservation 调整 Port、Commit Barrier、决策诊断。
- **验收：** Commit 后绝不换厂商；更贵目标未经预算调整不执行；Tools/Reasoning/State 不兼容目标被拒绝。
- **验证：** Commit 前后故障注入、预算失败、能力不等价、取消和调用次数测试。
- **不包含：** 真实 Ledger 实现，使用可验证 Reservation Port。

### Task 43：收紧 Cache 与 Singleflight 语义

- **状态：** Pending
- **依赖：** Task 25、Task 38、Task 42
- **目标：** 缓存和请求合并不能跨租户或改变有状态/工具请求语义。
- **交付：** Canonical Cache Key、eligibility policy、租户/策略 revision 隔离、共享调用取消规则、cache source Usage hook。
- **验收：** Tool、vendor-managed state、敏感或不可确定请求默认不缓存；跨租户永不命中；全部订阅者取消才取消共享上游。
- **验证：** 隔离、取消、并发、流式和错误不缓存测试。
- **不包含：** 语义缓存质量评测；企业模式默认关闭。

### Task 44：执行 M4 Exit Gate

- **状态：** Pending
- **依赖：** Task 37～Task 43
- **目标：** 证明路由与韧性在故障下仍保持请求语义和成本边界。
- **交付：** `docs/gates/M4.md`、故障矩阵和 Route Dry Run 演示。
- **验收：** 不兼容目标零调用；Commit 后零切换；取消零重试；Breaker 隔离正确；Fallback 预算/能力全部重验。
- **验证：** 全量 CI、race、故障注入、缓存隔离和端到端 Trace。
- **不包含：** M5 持久化账本。

---

## 10. M5：企业控制面、身份与账本

### Task 45：确定租户、安全、配置发布与 Ledger ADR

- **状态：** Pending
- **依赖：** Task 44
- **目标：** 在建库前固定租户、Credential、预算、账本和配置事实源不变量。
- **交付：** ADR 集：Organization/Project/RBAC、Key/HMAC、Envelope Encryption、Endpoint/SSRF、Reservation/Ledger、Config Revision/Outbox。
- **验收：** 事务、幂等、失败语义、迁移/回滚、SQLite standalone 与 PostgreSQL production 边界明确。
- **验证：** 威胁建模和并发时序审查。
- **不包含：** Schema 与代码。

### Task 46：建立 PostgreSQL、Migration 与 Repository Conformance

- **状态：** Pending
- **依赖：** Task 45
- **目标：** 生产事实源具备版本化 Schema，SQLite 仅保留 standalone 等价子集。
- **交付：** pgx/sqlc/goose 基础设施、核心表迁移、Repository Ports、PostgreSQL/SQLite Conformance、事务测试容器。
- **验收：** Migration 幂等且可前滚；两实现关键语义一致；网络调用不进入数据库事务。
- **验证：** Migration Integration、Repository Conformance、连接失败/readiness 测试。
- **不包含：** 一次完成所有业务 Repository 方法。

### Task 47：实现 Organization、Project 与 RBAC 边界

- **状态：** Pending
- **依赖：** Task 46
- **目标：** 所有管理和数据事实具备明确租户作用域。
- **交付：** 组织/项目/成员/角色模型、Repository、Identity Port、开发模式 bootstrap、授权策略。
- **验收：** Repository 方法显式携带 scope；跨租户读写/列表/导出全部拒绝；生产模式不允许隐式本地管理员。
- **验证：** RBAC 矩阵、跨租户反向和审计主体测试。
- **不包含：** 完整外部 OIDC Provider UI，可先实现标准接口与受控开发登录。

### Task 48：实现 Gateway API Key 生命周期

- **状态：** Pending
- **依赖：** Task 46、Task 47
- **目标：** Key 可定位、可轮换、可撤销且不明文落库。
- **交付：** `key_id + high-entropy secret` 格式、HMAC 摘要、常数时间校验、scope/model policy、创建/禁用/轮换/最后使用时间。
- **验收：** 明文只在创建时返回一次；禁用/轮换即时生效；缓存不会延迟越权；数据库与日志无 secret。
- **验证：** 生命周期、并发禁用、时间比较、跨租户和 Secret Scan。
- **不包含：** Provider Credential。

### Task 49：实现 Provider Credential 加密与 Endpoint 安全

- **状态：** Pending
- **依赖：** Task 46、Task 47
- **目标：** 上游 Key 加密保存且只能发送到受控目标。
- **交付：** Envelope Encryption Port、开发主密钥/生产 KMS 接口、key version/rotation、Credential reference、Endpoint allowlist、Redirect/TLS/Proxy 策略。
- **验收：** 数据库/日志/API 无明文；解密失败 Fail Closed；任意 URL、恶意 Redirect、内网地址无法获取凭据。
- **验证：** 轮换、错误密钥、SSRF/Redirect、日志脱敏和权限测试。
- **不包含：** 绑定具体云 KMS 的所有实现，可先提供一个生产接口和一个开发实现。

### Task 50：实现版本化控制面配置与 Runtime Snapshot v2

- **状态：** Pending
- **依赖：** Task 16、Task 46～Task 49
- **目标：** Provider/Model/Route/Policy 配置通过草稿、校验、发布和 Outbox 成为事实源。
- **交付：** config revision、draft/publish/rollback、交叉校验、Outbox、数据面加载/通知 Port、Snapshot v2、乐观并发控制。
- **验收：** 发布一次原子生效；失败保留旧 revision；并发编辑不静默覆盖；Credential 只以 reference 进入 Snapshot。
- **验证：** 事务、冲突、失败回滚、并发请求、旧资源释放和 race 测试。
- **不包含：** Redis 通知实现，M6 先可轮询 Outbox/revision。

### Task 51：实现 Budget Reservation 原子预占

- **状态：** Pending
- **依赖：** Task 46、Task 47、Task 50
- **目标：** 并发请求不能在调用上游前穿透 Token/金额预算。
- **交付：** Budget/Reservation/Price Snapshot、整数/Decimal 金额、幂等 request ID、reserve/reject/adjust/release 事务。
- **验收：** 高并发总预占不超过预算；Store 故障 Fail Closed；Fallback 加价必须先成功 adjust。
- **验证：** 数据库并发、重复 request、边界金额、故障和 race/integration 测试。
- **不包含：** 最终 Usage 结算与冲正。

### Task 52：实现 Usage Settlement、不可变 Ledger 与 Reconcile

- **状态：** Pending
- **依赖：** Task 51
- **目标：** Unary、Stream、取消、缺失 Usage 和重复事件都可对账。
- **交付：** Usage Event、settle/partial/release/reversal/adjustment、append-only Ledger、estimated usage、reconcile worker、幂等 event ID。
- **验收：** 重复事件不重复扣费；取消/断流有确定状态；历史账单不受价格修改影响；修正不更新旧 Ledger 行。
- **验证：** 状态机、重复/乱序事件、缺 Usage、worker 重启、数据库故障和并发测试。
- **不包含：** 财务发票和支付系统。

### Task 53：实现 OpenAPI-first 控制面 API

- **状态：** Pending
- **依赖：** Task 47～Task 52
- **目标：** 用版本化契约管理租户、Key、Provider、Route、Budget、Ledger 和 Audit。
- **交付：** OpenAPI 3.1、生成 Gin 接口、DTO/Validation/Error Envelope、分页/幂等/revision、RBAC 和写操作审计。
- **验收：** Handler 不返回数据库实体/secret；Schema 与实现一致；越权、冲突和校验错误稳定。
- **验证：** OpenAPI lint/generation clean、Contract/Integration、RBAC 和负向测试。
- **不包含：** 管理端页面。

### Task 54：迁移 React 管理端到 v3 控制面

- **状态：** Pending
- **依赖：** Task 53
- **目标：** 管理端真实呈现 Provider Evidence、路由解释、Key、Budget、Ledger 和配置发布状态。
- **交付：** 生成/集中 API Types、Provider/Model/Route/Key/Budget/Audit 页面、Dry Run、revision conflict、Credential 一次性输入。
- **验收：** loading/empty/error/permission/conflict 完整；Credential 不回显；能力状态不由前端猜测；窄屏和键盘可用。
- **验证：** 组件/流程测试、权限/冲突/Secret 测试、lint/test/build。
- **不包含：** 纯装饰性大改版。

### Task 55：执行 M5 Exit Gate

- **状态：** Pending
- **依赖：** Task 45～Task 54
- **目标：** 证明多租户、安全、配置发布和账务不变量成立。
- **交付：** `docs/gates/M5.md`、威胁检查、并发额度报告、Migration/Recovery 和管理端演示。
- **验收：** 跨租户全部拒绝；Key/Credential 不明文；并发额度不穿透；重复 Usage 不重复扣费；配置原子发布。
- **验证：** 全量 CI、race、PostgreSQL Integration、Secret Scan、OpenAPI Contract、前端流程与故障测试。
- **不包含：** 多实例一致性声明。

---

## 11. M6：多实例、真实调用方与最终发布

### Task 56：实现 Redis 分布式协调与降级语义

- **状态：** Pending
- **依赖：** Task 55
- **目标：** 两实例共享必要临时状态，同时不把事实数据迁入 Redis。
- **交付：** 分布式 Rate Limit、配置 revision 通知、可选精确缓存/健康摘要、每项 Fail Open/Closed、readiness 和恢复诊断。
- **验收：** Key/Budget/Ledger/Audit 不只存在 Redis；Redis 故障行为按能力分别确定；恢复后无重复发布或额度绕过。
- **验证：** 两实例并发、Redis 断开/恢复、限流一致性、通知丢失后轮询修复测试。
- **不包含：** 用 Redis 替代 PostgreSQL 事实源。

### Task 57：建立两实例 Compose 与部署生命周期

- **状态：** Pending
- **依赖：** Task 56
- **目标：** 用可复现环境验证迁移、readiness、滚动重启和长流关闭。
- **交付：** 多阶段 Dockerfile、Compose（2 Gateway + PostgreSQL + Redis）、初始化/迁移、health/readiness、持久卷、Secret 注入和运维命令。
- **验收：** 一条命令启动；两实例 revision 一致；单实例重启不中断全部服务；Schema 不兼容时 readiness 拒绝。
- **验证：** Image build、Compose Smoke、滚动重启、迁移失败、graceful shutdown。
- **不包含：** Kubernetes；Compose 验收完成后再决定。

### Task 58：接入 MovieInsight 真实调用方

- **状态：** Pending
- **依赖：** Task 57
- **目标：** 用现有业务验证客户端兼容、流式、路由、Usage 和审计闭环。
- **交付：** 集成配置、最小代码改动、端到端场景、请求 Timeline 和脱敏使用报告。
- **验收：** 调用方只替换受控 Base URL/Key；正常、限流、上游失败和切换场景可重现；无业务秘密进入报告。
- **验证：** 真实端到端回归和至少一段持续运行记录。
- **不包含：** 为适配网关重写 MovieInsight 业务架构。

### Task 59：接入 Deep Research 真实调用方

- **状态：** Pending
- **依赖：** Task 57、Task 58
- **目标：** 用长流程、Tool/Reasoning 和取消场景验证协议语义。
- **交付：** 集成、长流/工具场景、取消/超时/恢复报告、与 MovieInsight 不同的能力需求证明。
- **验收：** Tool/Reasoning 状态不丢失；长流取消释放资源并结算；Planner 能解释目标选择与拒绝。
- **验证：** 真实多轮端到端、故障注入和 Usage 对账。
- **不包含：** 把 Deep Research 工作流搬进网关。

### Task 60：建立可复现 Load、Soak 与资源基线

- **状态：** Pending
- **依赖：** Task 57～Task 59
- **目标：** 性能数字来自脚本与原始结果，而不是简历估计。
- **交付：** `benchmarks/` 环境说明、Unary/SSE/混合负载、500 流连接、30 分钟 Soak、pprof/metrics、原始结果和分析。
- **验收：** 可重复运行；区分网关开销和上游延迟；无持续 goroutine/连接/内存增长；结果不含内容或秘密。
- **验证：** v3 性能目标对应的完整脚本执行与二次复跑。
- **不包含：** 为追数字关闭正确性、安全或可观测能力。

### Task 61：执行系统级 Fault、Recovery 与安全验证

- **状态：** Pending
- **依赖：** Task 56～Task 60
- **目标：** 证明依赖故障、部分失败和恢复不会破坏语义、租户或账务。
- **交付：** PostgreSQL/Redis/Provider/网络/磁盘/实例故障矩阵、恢复时间、NeedsReconcile、Secret/SSRF/越权复测。
- **验收：** Commit 后无静默切换；预算不穿透；配置不分裂；恢复后无重复结算；readiness 与真实能力一致。
- **验证：** 自动 Fault Suite、两实例长时间运行和恢复报告。
- **不包含：** 无证据声称跨地域灾备。

### Task 62：执行最终 Exit Gate 与 v3 Release

- **状态：** Pending
- **依赖：** Task 56～Task 61
- **目标：** 以可审计证据完成最终项目，而不是以功能列表宣布完成。
- **交付：** `docs/gates/M6-final.md`、Release Notes、部署/升级/回滚/备份/恢复/Security Runbook、API/配置迁移指南、最终能力矩阵和简历表述。
- **验收：** M0～M6 Gate 全部通过；两实例与两个真实调用方持续接入；性能和故障结果可复现；README 只描述 verified 能力。
- **验证：** 全量 CI、race、Conformance、三厂商 Smoke、Compose、Migration、Fault、Load/Soak、Secret Scan 和文档命令复跑。
- **不包含：** 未进入设计范围的 Realtime、Batch、Agent 编排和 Kubernetes Operator。

---

## 12. 需求到 Task 的追踪矩阵

| v3 能力 | 负责 Task | 最终证据 |
| --- | --- | --- |
| 可复现构建、配置、安全、CI | 1～10 | M0 Gate |
| Gin 双平面、核心解耦、Snapshot v1 | 11～18 | M1 Gate |
| Canonical IR、Chat/Responses、typed SSE | 19～27 | M2 Gate |
| 方舟、DeepSeek、Qwen Native Dialect | 28～36 | M3 Gate + Smoke Matrix |
| Capability Planner、Retry/Breaker/Fallback | 37～44 | M4 Fault Matrix |
| 多租户、Credential、配置发布、Ledger | 45～55 | M5 Security/Quota Report |
| Redis、两实例、真实调用方、性能与恢复 | 56～62 | M6 Final Report |

## 13. 最终完成判定

项目只有在 Task 62 为 `Done` 时才称为 v3 最终完成。代码量、页面数量、接入厂商数量或单次 Demo 都不能替代以下证据：

1. 所有 Task 的依赖和 Exit Gate 均已完成，没有跳项。
2. 客户端协议兼容与三家真实上游验证均有合约和实测证据。
3. Tools、Reasoning、Structured Output、Multimodal、Usage、State 和流错误不会静默丢失。
4. 多租户、Credential、预算和 Ledger 的安全/并发不变量通过故障测试。
5. 两实例、两个真实调用方、Load/Soak/Fault 和恢复报告可复现。
6. README、Release 和简历表述不超过实际 verified 能力。
