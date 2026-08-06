# 校准 README 与跨平台运行说明

- 日期：2026-08-06 15:44 CST
- 类型：L1
- 关联需求：Task 9
- 影响模块：`README.md`、`docs/build.md`、`.gitattributes`、`start.sh`、`stop.sh`、v3 任务书

## 根因

原 README 以“核心特性”平铺当前代码和目标能力，没有 Implemented、Experimental、Planned 与 Unverified 事实等级；遗留 OpenAI-compatible 文本转发容易被误读为国内厂商已完成语义适配。配置摘要还把实际默认关闭的 Quota 写成开启，并把受 AdminOnly 保护的静态 `/admin/health` 混同为可用健康证据。

仓库没有 Dockerfile/Compose 或容器 Smoke，README 却给出了无证据的部署配方。`start.sh` 仍从已停用的 `web/dist` 复制产物，`start.sh`/`stop.sh` 还会按端口终止任意进程，无法证明它们只操作本项目资源。

## 解决方案

- 重写 README 能力矩阵，为构建、Ingress、遗留 Egress、国内三厂商、Store/Quota、Admin 和 v3 目标标注事实等级与证据边界。
- 将客户端 Chat Completions Ingress 与真实上游厂商适配分开表述，明确 Ark、DeepSeek、Qwen 只有 disabled/unverified Bootstrap Schema，Native Adapter 尚未实现。
- 替换仓库地址、工具链、安全示例、跨平台构建/启动、Secret 契约、HTTP 边界和已知限制，删除未验证 Docker 配方。
- `start.sh` 改为调用统一后端构建入口并以前台 `exec` 启动；`.gitattributes` 将 `*.sh` 固定为 LF，避免 Windows 检出损坏 Bash 入口；删除会按端口强杀进程的 `stop.sh`，前台进程使用 `Ctrl+C` 优雅退出。

## 行为与兼容性

Gateway API、配置、数据库和运行时请求语义不变。Unix 启动辅助流程从“构建前端、复制双份产物、后台运行、按端口杀进程”收敛为“使用已跟踪的前端产物构建后端，前台运行”。需要后台托管的用户应使用明确持有 PID 的 Supervisor，不再依赖端口扫描。

Task 9 标记为 Done，Task 10 释放为唯一 Ready 任务。容器部署仍属于 Task 58，国内三厂商真实能力仍属于 M3。

## 可观测性

未修改运行时日志、指标或 Trace。文档现在明确说明 `/admin/health` 是受保护的静态响应而非 readiness，避免运维系统将它误用为依赖健康证据。

## 验证

- `[通过]` Windows `go run ./cmd/build`：执行干净 `npm ci`、TypeScript/Vite Build 和 Gateway Build，`internal/static/dist` 无差异。
- `[通过]` Windows 默认配置启动 Smoke：管理 SPA 返回 200，无 admin 身份的 `/admin/health` 返回 403，Job 退出后无残留监听。
- `[通过]` WSL `bash ./start.sh`正向 Smoke：管理 SPA 200、`/admin/health` 403；Job 停止后无残留监听。
- `[通过]` `bash -n ./start.sh`、缺失配置负向 Probe 和 `git check-attr eol -- start.sh`。
- `[通过]` README/Build 本地链接、Markdown 代码围栏、仓库 URL、实际配置值和禁止声明检查；`git ls-remote` 证明 Clone URL 可访问。
- `[通过]` `go run ./cmd/quality`、`go test -count=1 ./...`、`go vet ./...`。
- `[通过]` `npm run quality`：ESLint、2 个测试文件/6 个测试、TypeScript 和 Vite Build。
- `[通过]` Gitleaks v8.27.2、`git diff --check` 与生成产物差异检查。

## 风险与回滚

主要风险是后续能力实现后 README 再次滞后；后续 Task 必须在取得对应 Gate/Smoke 证据后同步升级事实等级，不能只修改形容词。回滚可恢复 README、构建文档和 Unix 脚本；被删除的 `stop.sh` 可通过 Git 恢复，但会重新引入按端口终止非本项目进程的风险。无数据迁移。用户已有的 v2 设计文档删除不属于本任务，不会暂存或提交。
