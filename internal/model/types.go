package model

import (
	"context"
	"encoding/json"
)

type Message struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

type ContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   any             `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
	Strict      bool           `json:"strict,omitempty"`
}

type Request struct {
	Model     string           `json:"model"`
	MaxTokens int              `json:"max_tokens"`
	System    string           `json:"system,omitempty"`
	Messages  []Message        `json:"messages"`
	Tools     []ToolDefinition `json:"tools,omitempty"`
}

type Response struct {
	ID         string         `json:"id"`
	StopReason string         `json:"stop_reason"`
	Content    []ContentBlock `json:"content"`
}

type Client interface {
	CreateMessage(ctx context.Context, req Request) (Response, error)
}
