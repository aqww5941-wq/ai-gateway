# Bootstrap 配置契约

本文描述当前 M0 阶段 `config.Load` 接受的 Bootstrap YAML。Task 4 已为火山方舟、
DeepSeek、Qwen 建立彼此独立的 Native Provider 声明；三家 Adapter 尚未实现，因此这些
声明只能是 `enabled: false`、`evidence.status: unverified`。

## 加载边界

一次加载严格执行以下顺序，任一步失败都不会返回部分配置：

1. 读取一个 YAML 文件。
2. 展开普通字段中的 `$NAME` 或 `${NAME}` 环境变量；变量不存在时一次报告排序后的名称。
3. 使用 `yaml.v3` `KnownFields` 解码，拒绝未知字段和多 YAML 文档。
4. 对未出现的字段应用集中默认值。显式 `0`、`0s` 或空字符串不会被默认值覆盖。
5. 按确定顺序执行字段范围、枚举、重复标识和交叉引用校验。

`credential.env` 保存的是 Secret 环境变量名称（例如 `DASHSCOPE_API_KEY`），不是
`${DASHSCOPE_API_KEY}`，所以加载 Native Bootstrap 声明时不会把秘密值复制进 `Config`。
启动和配置 Reload 都调用同一个 `config.Load`。Reload 的原子发布与动态/静态字段分类不在
本契约中，由 Task 5 处理。

## 最小可运行的安全示例

```yaml
providers:
  - name: legacy-invalid-example
    type: openai
    api_key: invalid-example-provider-key
    base_url: https://example.invalid/v1
    models: [invalid-example-model]

routes:
  - name: default
    match: {model: invalid-example-model}
    strategy: round_robin
    targets:
      - provider: legacy-invalid-example
        model: invalid-example-model
```

`.invalid` 是保留域名，`invalid-example-provider-key` 是明显无效的占位值。该配置只能证明进程
组装和本地入口可运行，不能证明任何真实厂商能力。遗留 `type` / `api_key` / `base_url` 只在
Strangler Migration 期间保留，不得用来冒充 Ark、DeepSeek 或 Qwen Native Adapter。

## Native Provider Bootstrap Schema

三种 Kind 共用 `name`、显式 `enabled: false`、`credential.env`、`evidence.status`、`models`
和 `timeout`，但 Endpoint 字段进入各自命名空间，不能混用：

| Kind | 专属区块 | Endpoint 约束 | 默认示例 Credential reference |
| --- | --- | --- | --- |
| `ark` | `ark` | 北京地域、受控 `/api/v3`、独立 Chat/Responses protocol version、Endpoint ID | `ARK_API_KEY` |
| `deepseek` | `deepseek` | `global`、stable/beta 显式区分、Chat protocol version | `DEEPSEEK_API_KEY` |
| `qwen` | `qwen` | 北京 Workspace ID 必须与专属 Host 一致、Chat/Responses protocol version | `DASHSCOPE_API_KEY` |

Qwen 示例模型使用 `qwen3.7-flash`，Workspace ID 使用
`invalid-example-workspace-id`，因此示例没有真实调用权限。完整三家示例见
[`config/gateway.yaml`](../config/gateway.yaml)。

当前状态约束：

- `enabled` 必须显式为 `false`；设为 `true` 会在配置加载阶段失败并提示 Adapter 未实现。
- `evidence.status` 必须为 `unverified`；Mock、文档阅读或仅持有 Key 都不能把它改成 verified。
- `credential.env` 只接受大写环境变量名，不接受明文值或 `${...}` Secret 展开。
- 每个 Kind 只能出现对应厂商区块；Native 声明不能同时使用遗留 `type`、`api_key`、`base_url`。
- Route 不能引用 disabled Provider。仅包含 disabled Native 声明时允许 `routes: []`，但没有可运行上游。

## 集中默认值

| 字段 | 默认值 |
| --- | --- |
| `server.port` | `8081` |
| `server.read_timeout` / `write_timeout` | `30s` / `120s` |
| `server.db_path` | `data/gateway.db` |
| `server.max_concurrency` / `queue_size` / `queue_timeout` | `500` / `200` / `10s` |
| `server.transport.max_conns_per_host` | `100` |
| `server.transport.max_idle_conns_per_host` / `max_idle_conns` | `50` / `200` |
| OpenAI / Claude Provider `timeout` | `30s` / `60s` |
| Native Bootstrap Provider `timeout` | `30s`（仅声明，Adapter 未运行） |
| `rate_limit.per_key` / `per_model` | `60` / `100` |
| `cache.backend` / `strategy` / `ttl` | `memory` / `exact` / `1h` |
| `cache.max_size` / `threshold` | `1000` / `0.85` |
| `cache.redis_addr` | `localhost:6379` |
| `tracing.exporter` / `service_name` / `sample_ratio` | `stdout` / `ai-gateway` / `1.0` |
| `filter.mode` | `mask` |

## 校验不变量

- 端口在 `1..65535`；Duration 必须大于零；比例在 `(0, 1]`。
- Provider Name、Route Name 和 Route Match Model 不重复；Provider 内模型不重复。
- Native Endpoint 必须使用 Kind 对应的 HTTPS Host、Path、region 和 protocol version；Qwen Host
  还必须与 `workspace_id` 一致。
- Route Strategy 只能是 `round_robin`、`weighted`、`fallback`、`latency` 或
  `semantic`。
- 每个 Target 必须引用已声明 Provider 及其模型；Key Model Allowlist 必须引用已声明
  Route Model。
- `weighted`/`latency` Target Weight 必须大于零。
- `semantic` Route 必须恰好覆盖唯一的 `simple` 和 `complex` 规则，且不能同时配置普通
  Targets；其他策略不能携带被忽略的 Semantic Rules。
- Cache、Tracing、Filter、Auth Role 等枚举必须使用已支持值；无效值即使对应功能当前
  Disabled 也不会被静默保留。
- Base URL 必须是绝对 `http`/`https` URL，且不能携带 User Info、Query 或 Fragment。

## 稳定错误分类

`config.ConfigError` 提供以下 `ErrorKind`，调用方可通过 `config.ErrorKindOf` 分类，不需要
匹配错误字符串：

| Kind | 含义 |
| --- | --- |
| `read` | 文件无法读取；底层错误可通过 `errors.Is/As` 获取 |
| `environment` | 引用的环境变量不存在 |
| `parse` | YAML 语法、类型、未知字段或多文档错误 |
| `validation` | 字段范围、枚举、重复标识或交叉引用错误 |

错误只记录字段路径、非秘密标识和问题，不输出 API Key 或其他配置值。例如：

```text
config validation error at routes[0].targets[0].provider: references unknown provider "missing"
```

## 兼容性说明

升级后，过去把 DeepSeek、SiliconFlow 或豆包写成通用 `type: openai` 的默认示例已移除。
旧 OpenAI/Claude 配置仍可用于迁移期回归，但三家首批厂商必须改用新的 Kind Schema，并保持
disabled/unverified，直到对应 M3 Adapter 交付。过去被忽略的未知字段或构造阶段才暴露的错误会在 `config.Load` 直接失败。
`server.read_timeout` 和 `server.write_timeout` 不再是无效示例字段，实际用于 Go HTTP
Server。迁移旧配置时应先修复所有报告字段，不应删除校验或用零值绕过。
