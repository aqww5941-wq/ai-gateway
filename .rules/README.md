# `.rules` 维护说明

`.rules/` 保存 AI Gateway 的可执行工程规范。根目录 `AGENTS.md` 是唯一入口和路由索引，本目录不依赖文件名被 Agent 自动发现。

## 规则分层

- `00-project.md`：所有任务必读，提供项目边界、现状与信息源优先级。
- `01-engineering-principles.md`：所有修改必读，约束根因分析和实现质量。
- `02-delivery-workflow.md`：所有修改必读，定义验证、changelog 和 Git 交付。
- `03-observability.md`：所有运行时修改必读，定义日志、指标和 Trace。
- `scenarios/*.md`：按路径或变更类型选择性读取。

## 编写规则

1. 规则必须可执行，避免“保持高质量”这类无法判定的口号。
2. 每条重要规则尽量包含适用条件、正确示例、反例和验证方式。
3. 项目方向写入 `00-project.md` 或设计文档；不要在多个场景文件重复架构结论。
4. 新增场景规则时，同步更新根 `AGENTS.md` 的场景索引。
5. 规则与 Approved Baseline 冲突时，先确认设计是否变化，再同步修改两者。
6. 规则变更本身也走 `02-delivery-workflow.md`，生成 changelog 并提交。
