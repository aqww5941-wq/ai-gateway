# 固定 Golden Fixture 的跨平台换行

- 日期：2026-08-06 16:02 CST
- 类型：L1
- 关联需求：Task 10.R1
- 影响模块：`.gitattributes`、`config/testdata/*.golden`、v3 任务书

## 根因

M0 Exit Gate 首次使用 detached Windows 工作树验证提交 `c3a9cd9`。主工作树的 `unknown-field.golden` 保持 LF，而 `core.autocrlf=true` 下的全新检出包含 CRLF。`TestLoadErrorGolden` 对多行诊断做精确比较，实际错误使用 LF，因此只有新 Windows 工作树失败。Linux CI 和未重新检出的 Windows 主工作树都会掩盖该问题。

## 解决方案

`.gitattributes` 将 `*.golden` 显式固定为 `text eol=lf`。Golden Fixture 表达确定诊断文本，换行不是平台功能；因此在 Git 边界统一字节比在测试中容忍 CRLF/LF 更能保持 Golden 的精确性。不修改错误内容，不放宽断言。

## 行为与兼容性

Gateway 运行时、API、配置、数据库和错误文本均不变。只改变后续 Git 检出 Golden Fixture 时的换行字节，使 Windows 与 Linux 使用同一测试输入。

## 可观测性

无运行时日志、指标或 Trace 变化。失败证据保留在 M0 Gate 记录中，不把其改写为环境问题。

## 验证

- `[通过]` 主/全新工作树字节对比已证明根因：主工作树 `CR=0, LF=2`，旧全新检出 `CR=2, LF=2`。
- `[通过]` 从包含新规则的暂存区执行全新候选检出：Fixture `CR=0, LF=2`，`go test -count=1 ./...` 全部通过。
- `[通过]` 提交 `e7e4fd8` 后新建 detached Windows worktree：Fixture 仍为 `CR=0, LF=2`，完整构建、全仓 Test/Race/Vet/Staticcheck 与前端门禁全部通过。

## 风险与回滚

风险仅限后续 Windows 检出的 Golden 保持 LF，这与精确诊断契约一致。回滚 `.gitattributes` 规则会恢复全新 Windows 工作树的测试失败；无数据迁移。用户已有的 v2 设计文档删除不属于本任务，不会暂存或提交。
