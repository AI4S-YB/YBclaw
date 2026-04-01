# 模型配置

YBclaw 的模型接入由三个要素决定：provider、model、base URL / API key。

## Provider

通过 `-provider` 或环境变量 `CLAW_PROVIDER` 指定，目前支持：

- `anthropic`
- `openai-chat`
- `openai-responses`

## Model

通过 `-model` 或环境变量 `CLAW_MODEL` 指定。不显式指定时，各 provider 的默认值如下：

| provider | 默认模型 |
|----------|----------|
| `anthropic` | `claude-sonnet-4-7` |
| `openai-chat` | `gpt-5.4` |
| `openai-responses` | `gpt-5.4` |

## API Key 与 Base URL

各 provider 读取对应的环境变量：

| provider | API key | base URL |
|----------|---------|----------|
| `anthropic` | `ANTHROPIC_API_KEY` | `ANTHROPIC_BASE_URL` |
| `openai-chat` / `openai-responses` | `OPENAI_API_KEY` | `OPENAI_BASE_URL` |

也可以直接通过命令行参数 `-api-key` 和 `-base-url` 覆盖。

优先级：命令行参数 > provider 对应环境变量 > provider 默认值。

## Base URL 规则

程序会根据 provider 自动补全请求路径：

- `anthropic` → `.../v1/messages`
- `openai-chat` → `.../v1/chat/completions`
- `openai-responses` → `.../v1/responses`

关键规则：**如果 base URL 已经包含版本段（`/v1`、`/v2` 等类似前缀），不会再重复补 `/v1`，只补资源名。**

### 示例

base URL 不含版本前缀，程序补完整标准路径：

```
ANTHROPIC_BASE_URL=https://api.z.ai/api/anthropic
→ 实际请求 https://api.z.ai/api/anthropic/v1/messages
```

base URL 已含版本前缀，程序只补资源名：

```
ANTHROPIC_BASE_URL=https://api.z.ai/api/coding/paas/v4
→ 实际请求 https://api.z.ai/api/coding/paas/v4/messages
```
