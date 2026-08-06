# `.rules` 维护说明

`.rules/` 是 AI Gateway 的可执行工程规范；根 `AGENTS.md` 是唯一入口和场景路由表。Agent 不得依赖文件名自动发现规则。

## 规则分层

- `00-project.md`：所有任务必读，区分现状、目标、厂商范围和信息源。
- `01-engineering-principles.md`：所有修改必读，约束根因、职责、错误和并发。
- `02-delivery-workflow.md`：所有修改必读，约束验证、changelog 和 Git。
- `03-observability.md`：运行时代码必读，约束诊断证据和秘密保护。
- `scenarios/*.md`：由根 `AGENTS.md` 按路径和行为显式路由。

## 编写与维护

1. 规则必须可判定，写清适用条件、禁止项和验证方式，避免空泛口号。
2. 项目方向只在 `00-project.md` 与 v3 设计维护；场景文件只写该边界的执行细则。
3. 新增或重命名场景规则时同步更新根索引。
4. 规则与 v3 设计冲突时，先确认架构是否变化，再同步修改，不让两套事实并存。
5. 官方厂商行为引用一手文档；时效性能力必须记录验证日期、模型、地域和 API 版本。
6. 规则变更本身遵循 `02-delivery-workflow.md`，创建 changelog 和原子提交。
