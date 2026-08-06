# 修改、验证、Changelog 与 Git 交付

## 1. 开始任务

1. 运行 `git status --short` 和 `git branch --show-current`。
2. 把已有修改视为用户资产；不得清理、覆盖、回滚或顺带提交。
3. 阅读根规则、所有命中的场景规则、相关设计、入口、调用方和测试。
4. 先写根因、影响范围、方案和验证计划，再编辑。
5. 若当前实现与设计冲突，先判断是迁移中的已知差异还是设计变更，不自行猜测。

## 2. 实现要求

- Bug：先明确可证明根因的失败场景，再修复。
- 新功能：覆盖成功、失败、边界、取消和相关并发时序。
- 运行时决策：同步结构化日志，按影响增加指标与 Trace。
- API、配置、Schema、数据：同步契约、示例、迁移、兼容和回滚说明。
- Provider：同步 Capability、Golden、SSE Replay、Error Mapping 和 Conformance。
- 不把格式化、依赖升级或构建生成的无关变化混入任务。

## 3. 验证分级

Go 基础检查：

```text
gofmt（仅本次 Go 文件）
go test <受影响包>
go test ./...
go vet ./...
go build ./cmd/gateway
```

共享状态、Snapshot、Store、Cache、Router、Limiter、Quota 或流并发增加 `go test -race`。Provider 增加 Golden、SSE Replay、Adapter Conformance；Parser 增加 Fuzz Smoke。前端按仓库实际脚本执行依赖安装、lint、test 和 build，不存在的脚本不得写成已运行。

若工具未在 PATH，先查找仓库或系统已安装工具链；使用已确认的绝对路径执行。只有确实不存在或权限受限时才报告环境阻塞，不得用“环境问题”掩盖验证失败。

验证失败必须分类为本次回归、已知基线问题或真实环境限制，并保留原始命令和关键错误。未通过检查不得描述为通过。

## 4. 厂商验证分层

1. 离线：官方文档样例转为脱敏 Golden、错误 Fixture 和 SSE Replay，CI 必跑。
2. 合约：同一 Capability Suite 验证 Encode/Decode/Stream/Error/Usage，不允许只测 200 文本。
3. 真实 API：使用 `ARK_API_KEY`、`DEEPSEEK_API_KEY`、`DASHSCOPE_API_KEY` 的 opt-in Smoke Test；记录地域、Endpoint、模型、协议版本、日期和 Adapter revision。
4. 没有 Credential 时真实能力状态为 `unverified`，不能用 Mock 将其标记为 verified。Secret 不写入仓库、日志、Fixture 或测试报告。

## 5. Changelog

代码、配置、数据库、API、构建、测试行为或治理规则变化时创建：

```text
changelog/YYYYMMDD-HHmm-<lowercase-kebab-slug>.md
```

使用 Asia/Shanghai 本地时间和 `changelog/README.md` 模板，记录根因、方案、兼容/迁移、风险和实际验证。纯阅读、分析或无文件修改的审查通常不创建。

## 6. 原子 Git 提交

用户未禁止时默认创建本地提交，不 push。提交前：

1. 再次检查 status 和本任务 diff。
2. 使用 `git add -- <显式文件列表>`，禁止 `git add .` 或 `git add -A`。
3. 检查 `git diff --cached --check` 和完整 staged diff。
4. 确认未包含用户无关修改、秘密和生成噪音。

提交格式为 `<type>(<scope>): <imperative summary>`。默认不 push、force push、rebase、amend 或修改 Git 用户配置。存在验证失败、疑似秘密、重叠修改或无法区分归属时，保留工作树并说明阻塞，不强行提交。

## 7. 最终交付

最终回复说明：根因和结果、关键文件、可观测变化、实际验证、未执行项、changelog、commit SHA，以及用户原有修改是否保留。
