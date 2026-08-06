# 统一前端构建产物与跨平台构建入口

- 日期：2026-08-06 09:35 CST
- 类型：L1
- 关联需求：Task 2
- 影响模块：`cmd/build/`、`web/`、`internal/static/`、`Makefile`、`README.md`、构建文档

## 根因

前端构建原先先写入 `web/dist/`，再由 Makefile 使用 Unix 专用的 `rm` 和 `cp`
复制到 `internal/static/dist/`。两个目录同时被 Git 跟踪，无法判断哪一份是发布事实源，
Windows 也不能直接执行构建。仓库缺少换行属性约束，`core.autocrlf` 还会使 Windows
与 Unix 使用不同字节的 HTML 输入，破坏生成哈希的一致性。

## 解决方案

- 新增 Go 构建驱动，统一提供完整、前端、后端和清理四个跨平台目标。
- 让 Vite 直接清空并写入 `internal/static/dist/`，将其作为唯一跟踪产物和唯一
  `//go:embed` 输入；删除并忽略旧 `web/dist/`。
- 使用 `.gitattributes` 固定前端文本输入为 LF，并按原始字节跟踪嵌入资源。
- 后端构建固定 `CGO_ENABLED=0`、`-trimpath` 和 `-buildvcs=false`，按平台生成正确扩展名。
- Makefile 改为委托统一入口，README、Docker 示例和 `docs/build.md` 同步产物契约与
  Git 策略。
- Vite 开发代理与当前服务默认端口 `8081` 对齐。

Task 2 状态更新为 Done，Task 3 更新为唯一 Ready 项。

## 行为与兼容性

管理端页面功能和视觉不变。构建命令统一为 `go run ./cmd/build`；Windows 输出
`bin/gateway.exe`，Linux/macOS 输出 `bin/gateway`。`make` 仍可作为可选别名。
`go run ./cmd/build -target clean` 不删除已跟踪的嵌入资源，因此干净检出在没有 Node.js
时仍可直接构建 Go 网关。

## 可观测性

不适用。本次修改构建时行为和开发代理配置，没有新增或改变网关运行时决策。

## 验证

- `[通过]` `go test ./cmd/build`，覆盖平台二进制命名、仓库根识别、环境变量替换和
  清理边界。
- `[通过]` 连续两次 `go run ./cmd/build`；每轮均执行 `npm ci`、TypeScript/Vite
  build 和 Go build。
- `[通过]` 两次构建的 18 个 `internal/static/dist/` 文件 SHA-256 清单完全一致，
  第二次构建没有新增工作树变化。
- `[通过]` 清理目标删除 `bin/` 和旧 `web/dist/`，保留 `internal/static/dist/`；
  后端目标随后成功生成 `bin/gateway.exe`。
- `[通过]` `go test ./...`、`go vet ./...`。
- `[通过]` `git diff --check`、唯一嵌入路径和跟踪文件检查。
- `[未执行]` `go test -race ./...`；Task 1 已记录当前环境 `CGO_ENABLED=0` 且无 gcc，
  Task 2 不改变并发运行时代码，Race 门禁仍由 Task 8 处理。

## 风险与回滚

构建前端现在会直接清空规范产物目录；因此前端源码与生成产物必须在同一提交中更新。
回滚需要同时恢复旧输出路径、复制流程和两份产物，不涉及 API、配置、数据库或数据迁移。
用户此前删除的 v2 文档不纳入本任务提交。
