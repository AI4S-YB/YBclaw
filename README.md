# YBclaw

`YBclaw` 是自研的 claw 工具，作为后续项目的基建工作。

当前仓库提供的是一个最小可运行的 Go 版 agent core，先把最关键的工具调用闭环搭起来，便于后续持续扩展。

目标只保留三件事：

1. 把工具定义暴露给模型
2. 接收模型返回的 `tool_use`
3. 本地执行工具，再把 `tool_result` 回填给模型，直到拿到最终答案

刻意不做的部分：

- MCP
- Skill
- 子 agent / swarm
- UI / streaming transcript
- 权限系统
- hook / stop hook
- compact / memory / telemetry

## 当前实现结构

当前版本围绕最小 agent 闭环组织，核心模块如下：

- `internal/tools/tool.go`
  - 最小工具抽象
- `internal/tools/builtin.go`
  - 内置工具集合
- `internal/agent/agent.go`
  - 主循环：请求模型 -> 收集 `tool_use` -> 执行工具 -> 回填 `tool_result`
- `internal/model/anthropic.go`
  - Anthropic provider
- `internal/model/openai.go`
  - OpenAI Chat Completions / Responses provider

## 内置工具

- `list_files`
- `read_file`
- `write_file`
- `run_command`

这四个已经足够支撑一个最小 coding agent。

## 为什么选 Go

这版核心的工作主要是：

- 组织 JSON schema
- 调 Anthropic Messages API
- 执行本地命令
- 读写文件

Go 的标准库对这些场景最直接，单二进制也更适合作为后续 `Claw` 的核心进程。

## Provider

目前支持三种后端：

- `anthropic`
- `openai-chat`
- `openai-responses`

## 模型配置

`YBclaw` 的模型接入由三部分决定：

- provider
- model
- base URL / API key

### 1. provider

通过 `-provider` 或 `CLAW_PROVIDER` 指定：

- `anthropic`
- `openai-chat`
- `openai-responses`

### 2. model

通过 `-model` 或 `CLAW_MODEL` 指定具体模型名。

如果不显式指定，会按 provider 使用默认模型：

- `anthropic`
  - 默认：`claude-sonnet-4-7`
- `openai-chat`
  - 默认：`gpt-5.4`
- `openai-responses`
  - 默认：`gpt-5.4`

示例：

```bash
go run ./cmd/claw \
  -provider anthropic \
  -model claude-sonnet-4-7 \
  -prompt "当前的电脑配置如何"
```

```bash
go run ./cmd/claw \
  -provider openai-chat \
  -model gpt-5.4 \
  -prompt "列出当前目录文件"
```

### 3. API key 与 base URL

默认环境变量：

- `anthropic`
  - `ANTHROPIC_API_KEY`
  - `ANTHROPIC_BASE_URL`
- `openai-chat` / `openai-responses`
  - `OPENAI_API_KEY`
  - `OPENAI_BASE_URL`

公共覆盖项：

- `CLAW_PROVIDER`
- `CLAW_MODEL`

也可以直接通过命令行覆盖：

- `-api-key`
- `-base-url`

优先级大致如下：

- 显式命令行参数
- provider 对应环境变量
- provider 默认值

### 4. base URL 规则

程序会根据 provider 自动补全请求路径。

默认情况下：

- `anthropic` 会请求 `.../v1/messages`
- `openai-chat` 会请求 `.../v1/chat/completions`
- `openai-responses` 会请求 `.../v1/responses`

如果你的 `base URL` 已经带版本前缀，例如：

```bash
export ANTHROPIC_BASE_URL="https://api.z.ai/api/coding/paas/v4"
```

那么程序不会再重复补一个 `/v1`，而是会请求：

```text
https://api.z.ai/api/coding/paas/v4/messages
```

也就是说：

- 不带版本前缀：自动补完整标准路径
- 已带版本前缀：只补资源名，不重复补版本段

## Quick Start

先拉代码并进入项目目录：

```bash
git clone https://github.com/AI4S-YB/YBclaw.git
cd YBclaw
```

如果你要对接自定义代理或兼容网关，可以通过 `-base-url` 或对应的 `*_BASE_URL` 环境变量指定服务地址。

### Anthropic

```bash

export ANTHROPIC_API_KEY=your_key
export ANTHROPIC_BASE_URL=https://api.anthropic.com

go run ./cmd/claw \
  -provider anthropic \
  -prompt "看看 README，总结这个项目在做什么"
```

带版本前缀的兼容网关示例：

```bash
export ANTHROPIC_BASE_URL="https://api.z.ai/api/coding/paas/v4"

go run ./cmd/claw \
  -provider anthropic \
  -prompt "当前的电脑配置如何"
```

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

## 命令行参数

- `-prompt`
- `-provider`
- `-workdir`
- `-model`
- `-max-turns`
- `-max-tokens`
- `-base-url`
- `-api-key`
- `-quiet-tools`

## Git 约定

仓库提供了 `commit-msg` hook，用来限制提交信息为 Conventional Commits 风格。

先执行一次：

```bash
bash scripts/install-hooks.sh
```

支持的类型包括：

- `feat`
- `fix`
- `docs`
- `style`
- `refactor`
- `test`
- `chore`
- `build`
- `ci`
- `perf`
- `revert`

合法示例：

- `feat(agent): add tool loop`
- `chore: add commit hooks`
- `docs: update README`

## 当前边界

这是一个最小 agent 内核，不是完整产品形态。

它已经具备：

- 多轮 tool-use 闭环
- Anthropic / OpenAI Chat / OpenAI Responses 三种模型接入
- 工具注册与 schema 暴露
- 本地文件/命令工具
- workspace 边界检查
- 可测试的 model client 抽象

但还没有：

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
