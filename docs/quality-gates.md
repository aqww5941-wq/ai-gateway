# 质量门禁与本地等价命令

本文定义 Task 8 引入的最低质量门槛。GitHub Actions 工作流位于 `.github/workflows/quality.yml`，对 `push`、`pull_request` 和手工触发执行。依赖缓存只减少下载时间，不改变 `npm ci`、`go test -count=1` 或构建的输入和判断。

## 1. 工具链与职责

| 工具 | 固定方式 | 职责 | 运行时影响 |
| --- | --- | --- | --- |
| Go 1.26.4 | `go.mod` | Test、Race、Vet、Build | 项目工具链 |
| Staticcheck v0.7.0 | CI/本地 `go run ...@v0.7.0` | Go 语义静态分析 | 不写入 `go.mod`，不进入二进制 |
| Actionlint v1.7.12 | CI/本地 `go run ...@v1.7.12` | GitHub Actions 语法和表达式检查 | 仅开发/CI |
| Node 22.14.0 / npm 10.9 | `web/.node-version`、`packageManager`、lockfile | 前端确定性依赖与脚本环境 | 仅构建管理端 |
| ESLint 10 + TypeScript ESLint | `web/package-lock.json` | TypeScript、React Hooks、Fast Refresh 静态规则 | devDependency，不进入浏览器 Bundle |
| Vitest 4 | `web/package-lock.json` | 复用 Vite 转换链的最小单元测试框架 | devDependency，不进入浏览器 Bundle |
| Gitleaks v8.27.2 | CI/本地 `go run ...@v8.27.2` | 当前工作树 Credential 检测，输出强制脱敏 | 仅开发/CI |

选择 ESLint 而不是同时引入另一套 formatter/linter，是为了保留 TypeScript、React Hooks 与 Fast Refresh 的成熟规则；选择 Vitest 而不是 Jest，是为了复用现有 Vite/TypeScript 解析链。Staticcheck、Actionlint 和 Gitleaks 使用固定版本的临时 Go Tool，不扩大 Gateway 的生产依赖图。上述工具均为开发工具；新增 npm 包采用其公开的开源许可，安装结果由 lockfile 固定。

Gitleaks 暂时固定在 v8.27.2，而不是跟随 `latest`。2026-07 已有上游报告指出 v8.30.1 默认规则存在漏报回归，升级前必须先重跑本节的正向与负向 Probe：<https://github.com/gitleaks/gitleaks/issues/2170>。

## 2. 后端本地门禁

在仓库根目录执行：

```bash
# 跨平台 gofmt 检查：忽略 CRLF/LF 差异，但拒绝结构性格式差异
go run ./cmd/quality

go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
CGO_ENABLED=0 go build -trimpath -buildvcs=false -o /tmp/ai-gateway ./cmd/gateway
```

Windows 原生 Go 没有启用 Race 所需的 CGO/GCC 时，使用 WSL 执行 Race；普通 Test、Vet、Staticcheck 和 Build 仍在 Windows 执行。仓库历史 Go 文件包含 CRLF，`cmd/quality` 在比较前只规范化换行，再用标准库 `go/format` 判断；它不会忽略缩进、空格、语法或其他真实格式差异。

需要修复格式时执行 `go run ./cmd/quality -write`。该命令使用同一检查器，并保留文件当前的 LF/CRLF checkout 风格，避免格式修复制造整文件换行噪音。

## 3. 前端本地门禁

```bash
cd web
npm ci
npm run quality
cd ..
git diff --exit-code -- internal/static/dist
```

`npm run quality` 顺序执行 ESLint、Vitest、TypeScript 和 Vite Build。测试当前覆盖格式化/标签映射与 Tailwind class 合并，目的是建立框架和失败出口；后续页面或流程变化必须增量加入 loading、empty、error、permission 和 conflict 等场景测试。

前端 Build 仍写入唯一受跟踪产物 `internal/static/dist`。质量门禁要求源码与产物同步；缓存命中不能跳过 `npm ci` 或 Build，也不能隐藏产物差异。

## 4. Secret Scan

```bash
go run github.com/zricethezav/gitleaks/v8@v8.27.2 dir . --no-banner --redact
```

扫描日志必须保持 `--redact`。真实 API Key、Authorization、Prompt/Response 或 PII 不得作为 Probe、Fixture 或失败输出。规则升级时先使用无效但高熵的合成 Credential 在临时文件中证明扫描返回非零，再删除 Probe 并对仓库执行正向扫描。

## 5. CI Job

| Job | 必跑步骤 | 失败条件 |
| --- | --- | --- |
| Go quality | Format、Test、全仓 Race、Vet、Staticcheck、Actionlint、Build | 任一步非零或存在未格式化 Go 文件 |
| Frontend quality | `npm ci`、lint、test、typecheck、build、产物/工作树检查 | lockfile、源码、测试或受跟踪产物不一致 |
| Secret scan | Gitleaks 当前工作树扫描 | 检测到 Credential-shaped 内容 |

工作流权限固定为 `contents: read`，三个 Job 相互独立；缓存只保存 Go/npm 下载内容。`npm ci` 仍以 lockfile 重建依赖，Go Test 使用 `-count=1`，因此缓存不能把失败结果变成成功。

## 6. Task 8 验证记录

- `[通过]` 本地正向：ESLint、6 个 Vitest、TypeScript、Vite Build、Staticcheck、Actionlint、Gitleaks。
- `[通过]` 生成产物校验：前端 Build 后 `internal/static/dist` 无差异。
- `[通过]` 负向 Format Probe：未格式化 Go 文件被 `cmd/quality` 检出；其单测同时证明 CRLF/LF 不造成误报。
- `[通过]` 负向 Test Probe：合成失败测试使 `go test` 返回非零。
- `[通过]` 负向 Build Probe：合成 TypeScript 类型错误使前端 Build 返回非零。
- `[通过]` 负向 Secret Probe：无效高熵合成 Credential 使 Gitleaks 返回非零且输出脱敏。
- `[通过]` 全仓 Test/Race/Vet/Staticcheck/Build、干净 `npm ci`、生成产物差异与最终 Gitleaks。
- `[待外部证据]` GitHub-hosted `Quality` Workflow 首次成功运行。当前 `master` 比 `origin/master` 超前多个本地提交，项目规则默认不 push；取得明确 push 授权并完成远端运行前，Task 8 保持 In Progress，Task 9 不释放。
