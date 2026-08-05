# 修改、验证、Changelog 与 Git 交付

## 1. 开始任务

1. 运行 `git status --short` 和 `git branch --show-current`。
2. 将已有修改视为用户资产；不得覆盖、清理、回滚或顺带提交。
3. 阅读 `AGENTS.md` 指定的核心规则和所有命中的场景规则。
4. 阅读相关需求编号、设计、入口代码、调用方和现有测试。
5. 在动手前确定根因、范围和验证计划。

## 2. 实现要求

- 修 Bug：先添加或明确能证明根因的回归场景，再修复。
- 新功能：覆盖主路径、失败路径、边界条件以及相关并发时序。
- 修改运行时决策：同步诊断日志，必要时增加指标和 Trace 属性。
- 修改 API、配置、Schema 或数据：同步契约、示例、兼容/迁移和回滚说明。
- 修改前端：验证加载、空数据、错误、权限、窄屏和真实后端契约。
- 不把格式化或构建产生的无关变化混入任务。

## 3. 分级验证

执行与风险成比例的检查，并在 changelog 中记录实际结果。

### Go 基础检查

```text
gofmt（仅本次修改的 Go 文件）
go test <受影响包>
go test ./...
go vet ./...
go build ./cmd/gateway
```

涉及共享状态、热重载、缓存、路由、限流、配额或 Store 并发时，增加：

```text
go test -race <受影响包或 ./...>
```

### 前端基础检查

```text
cd web
npm ci（依赖环境需要初始化时）
npm run build
```

项目尚未建立前端 lint/test 脚本时，不得声称已运行；新增脚本后按仓库实际命令执行。

### 契约与高风险检查

- Provider/协议：Golden、SSE Replay、Adapter Conformance。
- API/Schema：兼容性和迁移测试。
- 安全/账务：威胁检查、并发、故障和幂等测试。
- 构建/部署：工作树洁净性、镜像、启动和 readiness smoke test。

验证失败时先判断是本次回归、基线问题还是环境限制。未通过的强制检查不得被省略或描述成通过。

## 4. Changelog

代码、配置契约、数据库、API、构建行为或测试行为发生变化时，在 `changelog/` 创建一份 Markdown：

```text
changelog/YYYYMMDD-HHmm-<lowercase-kebab-slug>.md
```

使用 Asia/Shanghai 本地时间。模板见 `changelog/README.md`。记录应解释根因、方案和验证，不要逐行复述 diff。

以下情况通常不创建：纯阅读、分析、代码审查、无文件修改的诊断。只改文档时，如果文档本身是交付物或治理规则，也应创建。

## 5. 自动 Git 提交

用户未明确禁止提交时，完成修改后自动创建本地提交，无需再次询问。

提交前必须：

1. 再次运行 `git status --short`。
2. 检查 `git diff -- <本任务文件>`。
3. 使用 `git add -- <显式文件列表>`；禁止用 `git add .`、`git add -A` 将用户修改一并纳入。
4. 检查 `git diff --cached --check` 和 `git diff --cached`。
5. 确认 changelog 与代码在同一提交中。

提交格式：

```text
<type>(<scope>): <imperative summary>
```

常用 type：`feat`、`fix`、`refactor`、`test`、`docs`、`build`、`chore`。

默认禁止：

- 自动 push、force push、rebase 或 amend。
- 修改 Git 用户配置。
- 暂存与任务无关的文件。
- 在验证失败、发现疑似秘密或无法区分重叠修改时强行提交。

不能提交时，保留修改并明确说明阻塞原因、未通过检查和未提交文件。

## 6. 交付说明

最终回复必须简洁说明：

- 解决了什么根因。
- 修改了哪些关键模块。
- 新增了哪些日志/指标/Trace。
- 实际通过了哪些验证，哪些未执行或未通过。
- changelog 文件和 commit SHA。
- 是否保留了用户原有未提交修改。
