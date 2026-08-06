# 构建与生成产物契约

本文定义 AI Gateway 的可复现构建入口、前端生成产物归属和 Git 管理策略。

## 工具链要求

- 完整或前端构建：Go 1.26.4、Node.js 22.14.0、npm 10.9.x。
- 只构建后端：Go 1.26.4；仓库已跟踪 `internal/static/dist/`，不要求本机安装 Node.js。
- 版本事实源是 `go.mod`、`web/.node-version`、`web/package.json` 和 lockfile；不要在文档或
  临时脚本中维护另一套版本。

## 统一入口

在仓库根目录执行以下命令：

```bash
# 完整构建：前端依赖、前端产物、Go 二进制
go run ./cmd/build

# 分阶段构建
go run ./cmd/build -target frontend
go run ./cmd/build -target backend

# 清理忽略的本地产物，不删除已跟踪的嵌入资源
go run ./cmd/build -target clean
```

`make build`、`make build-frontend`、`make build-backend` 和 `make clean` 只是这些
跨平台命令的便捷别名，不再包含 Unix 专用的复制或删除逻辑。

构建完成后，Linux/macOS 使用 `./bin/gateway -config config/gateway.yaml`，Windows
PowerShell 使用 `.\bin\gateway.exe -config .\config\gateway.yaml`。Unix 也可以执行
`./start.sh [config-path]`；该包装器只构建后端并以前台 `exec` 方式启动，不会按端口终止
其他进程。停止前台网关使用 `Ctrl+C`。

## 产物归属

| 路径 | 角色 | Git 策略 |
| --- | --- | --- |
| `web/src/` | React 源码 | 跟踪 |
| `internal/static/dist/` | Vite 唯一输出，同时是 `//go:embed` 唯一输入 | 跟踪，不得手工修改 |
| `web/dist/` | 已停用的重复输出位置 | 忽略 |
| `bin/` | 本地 Go 二进制 | 忽略 |

跟踪 `internal/static/dist/` 是有意的：干净检出即使没有 Node.js，也能直接执行
`go build ./cmd/gateway`，并得到包含管理端的单文件二进制。前端源码发生变化时，必须
运行前端构建并在同一个提交中更新该目录。

## 可复现性约束

- 前端依赖必须使用 `npm ci` 和已提交的 `web/package-lock.json`。
- `.gitattributes` 将前端文本输入固定为 LF，并按原始字节跟踪嵌入产物，避免
  `core.autocrlf` 造成 Windows/Unix 哈希分歧。
- Vite 直接清空并写入 `internal/static/dist/`，不存在构建后的跨目录复制。
- 后端构建固定 `CGO_ENABLED=0`、`-trimpath` 和 `-buildvcs=false`，减少主机与工作树元数据对二进制的影响。
- 不允许手工编辑构建产物，也不允许重新引入第二份被跟踪的前端产物。

## 发布前验证

连续运行两次完整构建，分别计算 `internal/static/dist/` 中所有文件的 SHA-256；两次
清单必须相同。随后确认生成过程没有造成非预期工作树变化：

```bash
go run ./cmd/build
go run ./cmd/build
git diff --exit-code -- internal/static/dist
git status --short
```

Windows 可以使用 `Get-FileHash -Algorithm SHA256`，Linux/macOS 可以使用
`sha256sum` 生成清单。预期的源码或生成产物变更应在提交中明确出现，不能用忽略规则
隐藏。

## 质量门禁

构建成功不是完整质量结论。Go Format/Test/Race/Vet/Staticcheck、前端
lint/test/typecheck/build、Secret Scan 和 GitHub Actions 的本地等价命令统一记录在
[`docs/quality-gates.md`](quality-gates.md)。依赖或工作流调整时必须同步更新该文档和
`.github/workflows/quality.yml`，且构建后仍需确认 `internal/static/dist` 没有非预期差异。
