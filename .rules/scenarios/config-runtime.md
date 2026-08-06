# 配置契约与 Runtime Snapshot

## 1. 配置来源

- Bootstrap YAML/Env：监听、Store、Redis、KMS、日志、运行模式等启动必需项。
- Control Plane Store：Provider、Endpoint、Model、Virtual Model、Route、Policy、Budget 和 Feature Flag。
- Secret Source：环境变量、Secret Manager 或 KMS；秘密不进入普通配置导出。

同一字段只能有一个明确优先级。严格解析未知字段、类型、范围和交叉引用；无效配置在发布前拒绝，不用默认值掩盖。

## 2. 厂商配置

- Ark、DeepSeek、Qwen 使用独立 provider kind 和 schema，不共享含糊的 `openai_compatible` 配置。
- Endpoint 包含受控 Base URL、region、protocol version、credential reference、模型和 capability evidence。
- Qwen Workspace Endpoint、Ark region endpoint、DeepSeek stable/beta endpoint 分开配置；禁止通过请求参数注入任意 URL。
- 示例只使用明显无效占位符，不提供看似可用的固定 Key。

## 3. Snapshot 发布

1. 读取完整候选配置。
2. 展开非秘密引用并解析 Secret reference，但不记录值。
3. 构建 Router、Adapter Registry、Transport、Policy 和 Store 依赖。
4. 校验所有交叉引用、能力和安全约束。
5. 成功后生成单调 revision 并一次原子交换。
6. 失败保持旧 Snapshot 完整可用；不能部分 Reload。
7. 请求捕获一个 revision，生命周期内不混用；旧资源引用归零后关闭。

## 4. 动态与静态字段

每个字段标记 dynamic 或 restart-required。修改 restart-required 字段时发布应拒绝并给出明确原因，不能表面返回成功。配置发布、拒绝、回滚和旧资源清理都要可观测。

## 5. 测试

覆盖未知字段、缺失必填、环境变量缺失、非法 URL/region、重复 ID、悬空引用、能力矛盾、Snapshot 构建失败、并发读取、旧资源释放、revision 单调性和 Secret 脱敏。运行 race test 验证原子切换。
