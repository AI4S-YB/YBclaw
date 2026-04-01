package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const defaultOpenAIBaseURL = "https://api.openai.com"

type OpenAIChatClient struct {
	HTTPClient *http.Client
	BaseURL    string
	APIKey     string
}

type OpenAIResponsesClient struct {
	HTTPClient         *http.Client
	BaseURL            string
	APIKey             string
	previousResponseID string
}

func NewOpenAIChatClient(apiKey, baseURL string) *OpenAIChatClient {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultOpenAIBaseURL
	}
	return &OpenAIChatClient{
		HTTPClient: &http.Client{Timeout: 5 * time.Minute},
		BaseURL:    baseURL,
		APIKey:     apiKey,
	}
}

func NewOpenAIResponsesClient(apiKey, baseURL string) *OpenAIResponsesClient {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultOpenAIBaseURL
	}
	return &OpenAIResponsesClient{
		HTTPClient: &http.Client{Timeout: 5 * time.Minute},
		BaseURL:    baseURL,
		APIKey:     apiKey,
	}
}

func (c *OpenAIChatClient) CreateMessage(ctx context.Context, req Request) (Response, error) {
	payload, err := buildOpenAIChatRequest(req)
	if err != nil {
		return Response{}, err
	}

	endpoint, err := buildOpenAIURL(c.BaseURL, "/v1/chat/completions")
	if err != nil {
		return Response{}, err
	}

	var response openAIChatResponse
	if err := c.doJSON(ctx, endpoint, payload, &response); err != nil {
		return Response{}, err
	}
	return normalizeOpenAIChatResponse(response)
}

func (c *OpenAIResponsesClient) CreateMessage(ctx context.Context, req Request) (Response, error) {
	payload, err := c.buildRequest(req)
	if err != nil {
		return Response{}, err
	}

	endpoint, err := buildOpenAIURL(c.BaseURL, "/v1/responses")
	if err != nil {
		return Response{}, err
	}

	var response openAIResponsesResponse
	if err := c.doJSON(ctx, endpoint, payload, &response); err != nil {
		return Response{}, err
	}
	c.previousResponseID = response.ID
	return normalizeOpenAIResponsesResponse(response)
}

func (c *OpenAIResponsesClient) buildRequest(req Request) (openAIResponsesRequest, error) {
	payload := openAIResponsesRequest{
		Model:           req.Model,
		Instructions:    req.System,
		MaxOutputTokens: req.MaxTokens,
	}
	tools := normalizedToolDefinitions(req.Tools)
	if len(tools) > 0 {
		payload.Tools = toOpenAIResponsesTools(tools)
		payload.ToolChoice = "auto"
	}

	if c.previousResponseID != "" && len(req.Messages) > 0 {
		last := req.Messages[len(req.Messages)-1]
		if last.Role == "user" {
			delta, err := buildOpenAIResponsesDeltaInput(last)
			if err != nil {
				return openAIResponsesRequest{}, err
			}
			if len(delta) > 0 {
				payload.PreviousResponseID = c.previousResponseID
				payload.Input = delta
				return payload, nil
			}
		}
	}

	input, err := buildOpenAIResponsesFullInput(req.Messages)
	if err != nil {
		return openAIResponsesRequest{}, err
	}
	payload.Input = input
	return payload, nil
}

func (c *OpenAIChatClient) doJSON(ctx context.Context, endpoint string, payload any, out any) error {
	return doOpenAIJSON(c.httpClient(), ctx, endpoint, c.APIKey, payload, out)
}

func (c *OpenAIResponsesClient) doJSON(ctx context.Context, endpoint string, payload any, out any) error {
	return doOpenAIJSON(c.httpClient(), ctx, endpoint, c.APIKey, payload, out)
}

func (c *OpenAIChatClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 5 * time.Minute}
}

func (c *OpenAIResponsesClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 5 * time.Minute}
}

func doOpenAIJSON(client *http.Client, ctx context.Context, endpoint, apiKey string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request model: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return parseOpenAIError(resp.StatusCode, respBody)
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func buildOpenAIURL(base, endpoint string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(base))
	if err != nil {
		return "", fmt.Errorf("parse base URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid base URL: %q", base)
	}
	u.Path = path.Join("/", strings.TrimPrefix(u.Path, "/"), strings.TrimPrefix(endpoint, "/"))
	u.RawPath = ""
	return u.String(), nil
}

func parseOpenAIError(statusCode int, body []byte) error {
	type errorEnvelope struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
			Code    any    `json:"code"`
		} `json:"error"`
	}

	var envelope errorEnvelope
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Error.Message != "" {
		code := ""
		if envelope.Error.Code != nil {
			code = fmt.Sprintf(" (%v)", envelope.Error.Code)
		}
		return fmt.Errorf("openai API %d %s%s: %s", statusCode, envelope.Error.Type, code, envelope.Error.Message)
	}
	return fmt.Errorf("openai API %d: %s", statusCode, strings.TrimSpace(string(body)))
}

type openAIChatRequest struct {
	Model               string              `json:"model"`
	Messages            []openAIChatMessage `json:"messages"`
	Tools               []openAIChatTool    `json:"tools,omitempty"`
	ToolChoice          string              `json:"tool_choice,omitempty"`
	MaxCompletionTokens int                 `json:"max_completion_tokens,omitempty"`
}

type openAIChatMessage struct {
	Role       string               `json:"role"`
	Content    any                  `json:"content"`
	ToolCalls  []openAIChatToolCall `json:"tool_calls,omitempty"`
	ToolCallID string               `json:"tool_call_id,omitempty"`
}

type openAIChatTool struct {
	Type     string                 `json:"type"`
	Function openAIChatFunctionTool `json:"function"`
}

type openAIChatFunctionTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
	Strict      bool           `json:"strict,omitempty"`
}

type openAIChatToolCall struct {
	ID       string                 `json:"id,omitempty"`
	Type     string                 `json:"type"`
	Function openAIChatFunctionCall `json:"function"`
}

type openAIChatFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIChatResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content   json.RawMessage      `json:"content"`
			ToolCalls []openAIChatToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
}

func buildOpenAIChatRequest(req Request) (openAIChatRequest, error) {
	tools := normalizedToolDefinitions(req.Tools)
	messages, err := buildOpenAIChatMessages(req.System, req.Messages)
	if err != nil {
		return openAIChatRequest{}, err
	}
	payload := openAIChatRequest{
		Model:               req.Model,
		Messages:            messages,
		MaxCompletionTokens: req.MaxTokens,
	}
	if len(tools) > 0 {
		payload.Tools = toOpenAIChatTools(tools)
		payload.ToolChoice = "auto"
	}
	return payload, nil
}

func buildOpenAIChatMessages(system string, history []Message) ([]openAIChatMessage, error) {
	messages := make([]openAIChatMessage, 0, len(history)+1)
	if strings.TrimSpace(system) != "" {
		messages = append(messages, openAIChatMessage{
			Role:    "system",
			Content: system,
		})
	}

	for _, message := range history {
		switch message.Role {
		case "user":
			text := collectText(message.Content)
			if text != "" {
				messages = append(messages, openAIChatMessage{
					Role:    "user",
					Content: text,
				})
			}
			for _, block := range message.Content {
				if block.Type != "tool_result" {
					continue
				}
				messages = append(messages, openAIChatMessage{
					Role:       "tool",
					Content:    stringifyContent(block.Content),
					ToolCallID: block.ToolUseID,
				})
			}
		case "assistant":
			assistant := openAIChatMessage{
				Role:    "assistant",
				Content: nil,
			}
			if text := collectText(message.Content); text != "" {
				assistant.Content = text
			}
			for _, block := range message.Content {
				if block.Type != "tool_use" {
					continue
				}
				assistant.ToolCalls = append(assistant.ToolCalls, openAIChatToolCall{
					ID:   block.ID,
					Type: "function",
					Function: openAIChatFunctionCall{
						Name:      block.Name,
						Arguments: normalizeJSONArgument(block.Input),
					},
				})
			}
			if assistant.Content != nil || len(assistant.ToolCalls) > 0 {
				messages = append(messages, assistant)
			}
		default:
			return nil, fmt.Errorf("unsupported message role for chat completions: %s", message.Role)
		}
	}

	return messages, nil
}

func normalizeOpenAIChatResponse(response openAIChatResponse) (Response, error) {
	if len(response.Choices) == 0 {
		return Response{}, fmt.Errorf("openai chat response had no choices")
	}

	choice := response.Choices[0]
	content := make([]ContentBlock, 0, len(choice.Message.ToolCalls)+1)
	if text := parseOpenAIText(choice.Message.Content); text != "" {
		content = append(content, ContentBlock{
			Type: "text",
			Text: text,
		})
	}
	for _, toolCall := range choice.Message.ToolCalls {
		content = append(content, ContentBlock{
			Type:  "tool_use",
			ID:    toolCall.ID,
			Name:  toolCall.Function.Name,
			Input: json.RawMessage(normalizeJSONArgument([]byte(toolCall.Function.Arguments))),
		})
	}

	stopReason := choice.FinishReason
	if len(choice.Message.ToolCalls) > 0 {
		stopReason = "tool_use"
	}

	return Response{
		ID:         response.ID,
		StopReason: stopReason,
		Content:    content,
	}, nil
}

func toOpenAIChatTools(definitions []ToolDefinition) []openAIChatTool {
	tools := make([]openAIChatTool, 0, len(definitions))
	for _, definition := range definitions {
		tools = append(tools, openAIChatTool{
			Type: "function",
			Function: openAIChatFunctionTool{
				Name:        definition.Name,
				Description: definition.Description,
				Parameters:  definition.InputSchema,
				Strict:      definition.Strict,
			},
		})
	}
	return tools
}

type openAIResponsesRequest struct {
	Model              string                `json:"model"`
	Instructions       string                `json:"instructions,omitempty"`
	PreviousResponseID string                `json:"previous_response_id,omitempty"`
	Input              []any                 `json:"input,omitempty"`
	Tools              []openAIResponsesTool `json:"tools,omitempty"`
	ToolChoice         string                `json:"tool_choice,omitempty"`
	MaxOutputTokens    int                   `json:"max_output_tokens,omitempty"`
}

type openAIResponsesTool struct {
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
	Strict      bool           `json:"strict,omitempty"`
}

type openAIResponsesMessageInput struct {
	Role    string                         `json:"role"`
	Content []openAIResponsesInputTextPart `json:"content"`
}

type openAIResponsesInputTextPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type openAIResponsesFunctionCallOutput struct {
	Type   string `json:"type"`
	CallID string `json:"call_id"`
	Output string `json:"output"`
}

type openAIResponsesResponse struct {
	ID     string                      `json:"id"`
	Output []openAIResponsesOutputItem `json:"output"`
}

type openAIResponsesOutputItem struct {
	ID        string                             `json:"id"`
	Type      string                             `json:"type"`
	CallID    string                             `json:"call_id"`
	Name      string                             `json:"name"`
	Arguments string                             `json:"arguments"`
	Role      string                             `json:"role"`
	Content   []openAIResponsesOutputContentPart `json:"content"`
}

type openAIResponsesOutputContentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func buildOpenAIResponsesFullInput(history []Message) ([]any, error) {
	items := make([]any, 0, len(history))
	for _, message := range history {
		text := collectText(message.Content)
		if text == "" {
			continue
		}
		switch message.Role {
		case "user", "assistant", "system":
			items = append(items, openAIResponsesMessageInput{
				Role: message.Role,
				Content: []openAIResponsesInputTextPart{
					{Type: "input_text", Text: text},
				},
			})
		default:
			return nil, fmt.Errorf("unsupported message role for responses API: %s", message.Role)
		}
	}
	return items, nil
}

func buildOpenAIResponsesDeltaInput(message Message) ([]any, error) {
	items := make([]any, 0, len(message.Content)+1)
	if text := collectText(message.Content); text != "" {
		items = append(items, openAIResponsesMessageInput{
			Role: message.Role,
			Content: []openAIResponsesInputTextPart{
				{Type: "input_text", Text: text},
			},
		})
	}
	for _, block := range message.Content {
		if block.Type != "tool_result" {
			continue
		}
		items = append(items, openAIResponsesFunctionCallOutput{
			Type:   "function_call_output",
			CallID: block.ToolUseID,
			Output: stringifyContent(block.Content),
		})
	}
	return items, nil
}

func toOpenAIResponsesTools(definitions []ToolDefinition) []openAIResponsesTool {
	tools := make([]openAIResponsesTool, 0, len(definitions))
	for _, definition := range definitions {
		tools = append(tools, openAIResponsesTool{
			Type:        "function",
			Name:        definition.Name,
			Description: definition.Description,
			Parameters:  definition.InputSchema,
			Strict:      definition.Strict,
		})
	}
	return tools
}

func normalizeOpenAIResponsesResponse(response openAIResponsesResponse) (Response, error) {
	content := make([]ContentBlock, 0, len(response.Output))
	for _, item := range response.Output {
		switch item.Type {
		case "function_call":
			id := item.CallID
			if id == "" {
				id = item.ID
			}
			content = append(content, ContentBlock{
				Type:  "tool_use",
				ID:    id,
				Name:  item.Name,
				Input: json.RawMessage(normalizeJSONArgument([]byte(item.Arguments))),
			})
		case "message":
			for _, part := range item.Content {
				if part.Type == "output_text" && strings.TrimSpace(part.Text) != "" {
					content = append(content, ContentBlock{
						Type: "text",
						Text: part.Text,
					})
				}
			}
		}
	}

	stopReason := "stop"
	for _, block := range content {
		if block.Type == "tool_use" {
			stopReason = "tool_use"
			break
		}
	}

	return Response{
		ID:         response.ID,
		StopReason: stopReason,
		Content:    content,
	}, nil
}

func collectText(blocks []ContentBlock) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			parts = append(parts, strings.TrimSpace(block.Text))
		}
	}
	return strings.Join(parts, "\n")
}

func parseOpenAIText(raw json.RawMessage) string {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return ""
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}

	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			if (part.Type == "text" || part.Type == "output_text") && strings.TrimSpace(part.Text) != "" {
				out = append(out, strings.TrimSpace(part.Text))
			}
		}
		return strings.Join(out, "\n")
	}

	return strings.TrimSpace(string(raw))
}

func normalizeJSONArgument(raw []byte) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return "{}"
	}
	var compact bytes.Buffer
	if json.Valid(trimmed) && json.Compact(&compact, trimmed) == nil {
		return compact.String()
	}
	return string(trimmed)
}

func stringifyContent(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case json.RawMessage:
		return string(typed)
	case []byte:
		return string(typed)
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprintf("%v", typed)
		}
		return string(data)
	}
}
