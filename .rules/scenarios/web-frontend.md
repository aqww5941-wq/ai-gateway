# React 管理端规则

## 1. 契约与职责

- 管理端只消费版本化控制面 API，不复制 Provider 能力、权限、价格或路由决策逻辑。
- TypeScript 类型优先从 OpenAPI/Schema 生成或集中维护；不得在页面散落不一致 DTO。
- 后端返回 capability status、evidence、translation warning 和 config revision，前端负责解释展示，不自行推测“支持”。
- 任何 Credential 字段只允许创建/轮换时输入，读取接口永不回显明文。

## 2. 必须呈现的状态

- loading、empty、error、stale、permission denied、conflict/revision mismatch 和 partial dependency failure。
- Provider/模型显示 `verified/documented/experimental/unverified/unsupported`、地域、协议版本和最近实测时间。
- 配置发布显示 validation、published、restart required、rejected 和 rollback 状态。
- 路由 Dry Run 展示候选、拒绝原因和能力缺口，不只显示最终厂商。

## 3. 安全与可用性

- Token 不写 localStorage、URL、日志或错误上报；敏感输入默认遮罩且禁止意外回填。
- 权限不是隐藏按钮即可，后端必须授权；前端仍应明确提示无权限。
- 表单使用服务端同源 Schema 约束，提交冲突不能静默覆盖。
- 验证键盘操作、焦点、颜色对比、窄屏、长文本和中文错误信息。

## 4. 构建与测试

- 只编辑 `web/src` 等源文件，不手改 `web/dist` 或嵌入副本。
- 覆盖组件/流程的加载、失败、空数据、权限、revision conflict 和 Secret 不回显。
- 按实际 package scripts 运行 lint/test/build；构建产物只有一个生成源，构建后不产生未预期工作树变化。
