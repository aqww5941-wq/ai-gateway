# 建立后端、前端与安全质量门禁

- 日期：2026-08-06 14:44 CST
- 类型：L1
- 关联需求：Task 8
- 影响模块：`.github/workflows/quality.yml`、`cmd/build`、`internal/retry`、`web`、`docs`

## 根因

仓库此前没有 CI；Go Test/Vet/Build/Race 和 Secret Scan 依赖人工执行，前端只有 Build，没有 lint/test 脚本或测试框架。缓存、锁文件、生成产物和静态分析之间也没有自动一致性检查，因此新提交可以在本机看似可运行，但带着格式、类型、测试、Secret 或过期嵌入产物进入主线。

Staticcheck 基线还暴露两处具体问题：构建助手优先依赖已弃用的 `runtime.GOROOT()`，Retry 的 `net.Error` 分类写法存在冗余分支。前者的根因是 Windows 上通过绝对路径执行 Go 时 PATH 可能不可用；后者只是等价表达不够直接。

## 解决方案

新增 GitHub Actions `Quality` 工作流，分为 Go、Frontend、Secret 三个只读 Job。Go Job 执行 Format、全仓 Test/Race、Vet、Staticcheck、Actionlint 和可复现 Build；Frontend Job 使用锁文件执行 ESLint、6 个 Vitest、TypeScript、Vite Build 和生成产物检查；Secret Job 使用固定版本 Gitleaks 并强制脱敏。由于历史 Go 文件以 CRLF 保存，Format Checker 先统一换行再用标准库 `go/format` 比较，从根因上区分跨平台 EOL 与真实排版差异，不维护豁免名单。

前端增加固定 Node/npm 契约、ESLint Flat Config、Vitest 和 `quality` 脚本。构建助手改为优先使用 PATH，仅在同主机的绝对 Go 启动场景保留有说明的 GOROOT fallback；Retry 分类改为等价的直接 `errors.As` 返回。新增质量门禁文档，记录工具职责、版本、替代方案、缓存正确性和本地等价命令。

## 行为与兼容性

Gateway API、配置、数据库和运行时请求语义不变。Retry 修改语义等价；构建助手仍支持当前 PATH 缺失但由绝对 Go 路径启动的 Windows 环境。新增 npm 包均为 devDependency，不进入浏览器生产 Bundle；Staticcheck、Actionlint 和 Gitleaks 不写入项目 `go.mod`。

Task 8 当前保持 `In Progress`：本地正向/负向门禁已通过，但 `master` 比远端超前多个提交，且项目规则默认不 push。没有 GitHub-hosted Workflow 首次成功记录前，不把 Task 8 或 Task 9 标记为 Done/Ready。

## 可观测性

没有修改运行时日志、指标或 Trace。CI 日志提供阶段化失败位置；Secret Scan 使用 `--redact`，Probe 和测试不记录真实 Credential、Authorization 或业务内容。

## 验证

- `[通过]` `npm run quality`：ESLint、2 个测试文件/6 个测试、TypeScript 和 Vite Build。
- `[通过]` `go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...`。
- `[通过]` `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12`。
- `[通过]` `go run github.com/zricethezav/gitleaks/v8@v8.27.2 dir . --no-banner --redact`。
- `[通过]` Format、Test、Build、Secret 四类无效合成 Probe 均使对应门禁返回非零，Probe 已删除。
- `[通过]` `go run ./cmd/quality` 与 `go test -count=1 ./...`。
- `[通过]` WSL `go test -race -count=1 ./...`。
- `[通过]` `go vet ./...`、Staticcheck、Actionlint 和可复现 Gateway Build。
- `[通过]` 干净 `npm ci` 后 `npm run quality`；2 个测试文件/6 个测试通过。
- `[通过]` 前端 Build 后 `git diff --exit-code -- internal/static/dist`，无生成产物变化。
- `[通过]` 全仓 Gitleaks 与 `git diff --check`。
- `[待外部验证]` GitHub-hosted `Quality` Workflow 首次成功运行；未 push，不伪造结果。

## 风险与回滚

主要风险是 CI 时间增加、Pinned Tool 需要定期升级，以及 ESLint/Staticcheck 规则升级可能暴露新的既有问题。`npm ci` 还报告既有 Recharts 2.x 已弃用；Task 8 不进行页面依赖大版本迁移，后续前端功能 Task 应单独评估 Recharts 3 的兼容性。升级必须先复现并分类，不能通过全局关闭规则造绿。回滚需同时恢复工作流、前端开发依赖/lockfile、测试、文档和两处静态分析修正；不涉及数据迁移。用户已有的 v2 设计文档删除不属于本任务，不会暂存或提交。
