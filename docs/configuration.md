# Bootstrap 配置契约

本文描述当前 M0 阶段 `config.Load` 接受的 Bootstrap YAML。Provider Schema 仍是迁移期的
`openai`/`claude` 结构；火山方舟、DeepSeek、Qwen 的独立 Kind 与 Secret reference 将由
Task 4 收敛。

## 加载边界

一次加载严格执行以下顺序，任一步失败都不会返回部分配置：

1. 读取一个 YAML 文件。
2. 展开 `$NAME` 或 `${NAME}` 环境变量；变量不存在时一次报告排序后的名称。
3. 使用 `yaml.v3` `KnownFields` 解码，拒绝未知字段和多 YAML 文档。
4. 对未出现的字段应用集中默认值。显式 `0`、`0s` 或空字符串不会被默认值覆盖。
5. 按确定顺序执行字段范围、枚举、重复标识和交叉引用校验。

启动和配置 Reload 都调用同一个 `config.Load`。Reload 的原子发布与动态/静态字段分类不在
本契约中，由 Task 5 处理。

## 最小合法配置

```yaml
providers:
  - name: primary
    type: openai
    api_key: ${PROVIDER_API_KEY}
    base_url: https://example.invalid/v1
    models: [model-a]

routes:
  - name: default
    match: {model: model-a}
    strategy: round_robin
    targets:
      - provider: primary
        model: model-a
```

环境变量必须真实存在；将变量设置为空仍会在 `providers[0].api_key` 校验阶段失败。

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
| `rate_limit.per_key` / `per_model` | `60` / `100` |
| `cache.backend` / `strategy` / `ttl` | `memory` / `exact` / `1h` |
| `cache.max_size` / `threshold` | `1000` / `0.85` |
| `cache.redis_addr` | `localhost:6379` |
| `tracing.exporter` / `service_name` / `sample_ratio` | `stdout` / `ai-gateway` / `1.0` |
| `filter.mode` | `mask` |

## 校验不变量

- 端口在 `1..65535`；Duration 必须大于零；比例在 `(0, 1]`。
- Provider Name、Route Name 和 Route Match Model 不重复；Provider 内模型不重复。
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

升级后，过去被忽略的未知字段或构造阶段才暴露的错误会在 `config.Load` 直接失败。
`server.read_timeout` 和 `server.write_timeout` 不再是无效示例字段，实际用于 Go HTTP
Server。迁移旧配置时应先修复所有报告字段，不应删除校验或用零值绕过。
