package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"claw/internal/model"
	"claw/internal/tools"
)

type Callbacks struct {
	OnAssistantText func(text string)
	OnToolCall      func(name string, input json.RawMessage)
	OnToolResult    func(name string, result tools.Result)
}

type Agent struct {
	Client    model.Client
	Tools     *tools.Registry
	Workspace string
	System    string
	Model     string
	MaxTokens int
	MaxTurns  int
	Callbacks Callbacks
}

type RunResult struct {
	Answer  string
	History []model.Message
}

func (a Agent) Run(ctx context.Context, prompt string) (RunResult, error) {
	if a.Client == nil {
		return RunResult{}, fmt.Errorf("missing model client")
	}
	if a.Tools == nil {
		return RunResult{}, fmt.Errorf("missing tool registry")
	}
	if strings.TrimSpace(a.Workspace) == "" {
		return RunResult{}, fmt.Errorf("missing workspace")
	}

	maxTurns := a.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 12
	}
	maxTokens := a.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	if strings.TrimSpace(a.Model) == "" {
		return RunResult{}, fmt.Errorf("missing model name")
	}

	history := []model.Message{
		{
			Role: "user",
			Content: []model.ContentBlock{
				{Type: "text", Text: prompt},
			},
		},
	}

	for turn := 0; turn < maxTurns; turn++ {
		response, err := a.Client.CreateMessage(ctx, model.Request{
			Model:     a.Model,
			MaxTokens: maxTokens,
			System:    a.systemPrompt(),
			Messages:  history,
			Tools:     toModelToolDefinitions(a.Tools.Definitions()),
		})
		if err != nil {
			return RunResult{}, err
		}

		assistantMessage := model.Message{
			Role:    "assistant",
			Content: response.Content,
		}
		history = append(history, assistantMessage)

		text := extractText(response.Content)
		if text != "" && a.Callbacks.OnAssistantText != nil {
			a.Callbacks.OnAssistantText(text)
		}

		toolResults := make([]model.ContentBlock, 0)
		toolCalls := 0
		for _, block := range response.Content {
			if block.Type != "tool_use" {
				continue
			}

			toolCalls++
			if a.Callbacks.OnToolCall != nil {
				a.Callbacks.OnToolCall(block.Name, block.Input)
			}

			result := a.Tools.Execute(ctx, tools.Env{Workspace: a.Workspace}, block.Name, block.Input)
			if a.Callbacks.OnToolResult != nil {
				a.Callbacks.OnToolResult(block.Name, result)
			}

			toolResults = append(toolResults, model.ContentBlock{
				Type:      "tool_result",
				ToolUseID: block.ID,
				Content:   result.Content,
				IsError:   result.IsError,
			})
		}

		if toolCalls == 0 {
			return RunResult{
				Answer:  text,
				History: history,
			}, nil
		}

		history = append(history, model.Message{
			Role:    "user",
			Content: toolResults,
		})
	}

	return RunResult{}, fmt.Errorf("max turns exceeded without final answer")
}

func (a Agent) systemPrompt() string {
	if strings.TrimSpace(a.System) != "" {
		return a.System
	}
	return DefaultSystemPrompt(a.Workspace)
}

func DefaultSystemPrompt(workspace string) string {
	return fmt.Sprintf(
		"You are Claw, a minimal coding agent.\n"+
			"Your job is to finish the user's task by reasoning, using tools, and then replying with a concise final answer.\n"+
			"Rules:\n"+
			"- Use tools when you need facts from the workspace or need to change files.\n"+
			"- Prefer inspecting files before editing them.\n"+
			"- Never invent tool results.\n"+
			"- Keep iterating until the task is complete or you are genuinely blocked.\n"+
			"- Workspace root: %s\n",
		workspace,
	)
}

func toModelToolDefinitions(definitions []tools.Definition) []model.ToolDefinition {
	out := make([]model.ToolDefinition, 0, len(definitions))
	for _, definition := range definitions {
		out = append(out, model.ToolDefinition{
			Name:        definition.Name,
			Description: definition.Description,
			InputSchema: definition.InputSchema,
			Strict:      definition.Strict,
		})
	}
	return out
}

func extractText(blocks []model.ContentBlock) string {
	parts := make([]string, 0)
	for _, block := range blocks {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			parts = append(parts, strings.TrimSpace(block.Text))
		}
	}
	return strings.Join(parts, "\n")
}
