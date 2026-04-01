# YBclaw

`YBclaw` 是自研的 claw 工具，作为后续项目的基建。

目标很简单，只做三件事：

1. 把工具定义暴露给模型
2. 接收模型返回的 `tool_use`
3. 本地执行工具，把 `tool_result` 回填给模型，直到拿到最终答案

刻意不做的部分：MCP、Skill、子 agent / swarm、UI / streaming transcript、权限系统、hook、compact / memory / telemetry。

## 项目结构

```
internal/
  tools/tool.go       # 工具抽象接口
  tools/builtin.go    # 内置工具
  agent/agent.go      # 主循环：请求 -> tool_use -> 执行 -> tool_result
  model/anthropic.go  # Anthropic provider
  model/openai.go     # OpenAI Chat Completions / Responses provider
cmd/claw/             # 入口
```

## 内置工具

- `list_files`
- `read_file`
- `write_file`
- `run_command`

四个工具已经足够支撑一个最小 coding agent。

## Quick Start

```bash
git clone https://github.com/AI4S-YB/YBclaw.git
cd YBclaw
```

### Anthropic

通过兼容网关也可以接入其他支持 Anthropic 协议的模型，例如智谱 GLM。

```bash
export ANTHROPIC_API_KEY=your_key
export ANTHROPIC_BASE_URL=https://api.z.ai/api/anthropic

go run ./cmd/claw \
  -provider anthropic \
  -prompt "看看 README，总结这个项目在做什么"
```

切换模型只需加 `-model`，例如 `-model glm-5.1`。

### OpenAI Chat Completions

```bash
export OPENAI_API_KEY=your_key

go run ./cmd/claw \
  -provider openai-chat \
  -model gpt-5.4 \
  -prompt "查看 README，并总结这个项目做了什么"
```

### OpenAI Responses API

```bash
export OPENAI_API_KEY=your_key

go run ./cmd/claw \
  -provider openai-responses \
  -model gpt-5.4 \
  -prompt "查看 README，并总结这个项目做了什么"
```

模型选择、base URL 配置、兼容网关接入等详见 [docs/model-config.md](docs/model-config.md)。

## 命令行参数

| 参数 | 说明 |
|------|------|
| `-prompt` | 用户输入 |
| `-provider` | 模型 provider（`anthropic` / `openai-chat` / `openai-responses`） |
| `-model` | 模型名 |
| `-workdir` | 工作目录 |
| `-max-turns` | 最大轮次 |
| `-max-tokens` | 最大 token 数 |
| `-base-url` | API base URL |
| `-api-key` | API key |
| `-quiet-tools` | 不打印工具调用过程 |

## 为什么选 Go

核心工作主要是组织 JSON schema、调 API、执行本地命令、读写文件。Go 标准库对这些场景最直接，单二进制也更适合作为后续 Claw 的核心进程。

## 当前边界

已具备：

- 多轮 tool-use 闭环
- Anthropic / OpenAI Chat / OpenAI Responses 三种模型接入
- 工具注册与 schema 暴露
- 本地文件/命令工具
- workspace 边界检查
- 可测试的 model client 抽象

暂未支持：

- 流式输出
- 并发工具调度
- 细粒度权限
- 中断恢复
- 长上下文压缩
- 专门的 edit/apply_patch 工具

## TODO

- 增加 `apply_patch` 风格编辑工具，避免只能整文件覆盖
- 增加 provider 级流式输出，支持实时展示模型回复和工具调用过程
- 增加更严格的命令权限控制，包括白名单、危险命令拦截和确认机制

## 相关文档

- [模型配置](docs/model-config.md) — provider、model、base URL 规则、兼容网关
- [Git 工作规范](docs/git-workflow.md) — Conventional Commits 格式与 hook 安装
