package model

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIChatClientMapsToolCalls(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if got, want := r.URL.Path, "/v1/chat/completions"; got != want {
			t.Fatalf("path mismatch: got %s want %s", got, want)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization header mismatch: %q", got)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}

		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}

		messages := payload["messages"].([]any)
		if callCount == 1 {
			if got := payload["tool_choice"]; got != "auto" {
				t.Fatalf("expected tool_choice auto, got %#v", got)
			}
			if len(messages) != 2 {
				t.Fatalf("expected 2 messages on first request, got %d", len(messages))
			}
			io.WriteString(w, `{
				"id":"chatcmpl_1",
				"choices":[
					{
						"finish_reason":"tool_calls",
						"message":{
							"content":"Checking the file.",
							"tool_calls":[
								{
									"id":"call_1",
									"type":"function",
									"function":{"name":"read_file","arguments":"{\"path\":\"README.md\"}"}
								}
							]
						}
					}
				]
			}`)
			return
		}

		lastMessage := messages[len(messages)-1].(map[string]any)
		if got := lastMessage["role"]; got != "tool" {
			t.Fatalf("expected final role tool, got %#v", got)
		}
		if got := lastMessage["tool_call_id"]; got != "call_1" {
			t.Fatalf("expected tool_call_id call_1, got %#v", got)
		}

		io.WriteString(w, `{
			"id":"chatcmpl_2",
			"choices":[
				{
					"finish_reason":"stop",
					"message":{"content":"README.md exists."}
				}
			]
		}`)
	}))
	defer server.Close()

	client := NewOpenAIChatClient("test-key", server.URL)

	first, err := client.CreateMessage(context.Background(), Request{
		Model:     "gpt-5.4",
		MaxTokens: 256,
		System:    "You are helpful.",
		Messages: []Message{
			{
				Role: "user",
				Content: []ContentBlock{
					{Type: "text", Text: "check README"},
				},
			},
		},
		Tools: []ToolDefinition{
			{
				Name:        "read_file",
				Description: "Read a file.",
				InputSchema: map[string]any{"type": "object"},
				Strict:      true,
			},
		},
	})
	if err != nil {
		t.Fatalf("first CreateMessage: %v", err)
	}
	if len(first.Content) != 2 || first.Content[1].Type != "tool_use" {
		t.Fatalf("unexpected first response content: %#v", first.Content)
	}
	if got := first.Content[1].ID; got != "call_1" {
		t.Fatalf("expected tool call id call_1, got %q", got)
	}

	_, err = client.CreateMessage(context.Background(), Request{
		Model:     "gpt-5.4",
		MaxTokens: 256,
		System:    "You are helpful.",
		Messages: []Message{
			{
				Role: "user",
				Content: []ContentBlock{
					{Type: "text", Text: "check README"},
				},
			},
			{
				Role:    "assistant",
				Content: first.Content,
			},
			{
				Role: "user",
				Content: []ContentBlock{
					{Type: "tool_result", ToolUseID: "call_1", Content: "README contents"},
				},
			},
		},
		Tools: []ToolDefinition{
			{
				Name:        "read_file",
				Description: "Read a file.",
				InputSchema: map[string]any{"type": "object"},
				Strict:      true,
			},
		},
	})
	if err != nil {
		t.Fatalf("second CreateMessage: %v", err)
	}
}

func TestOpenAIResponsesClientUsesPreviousResponseID(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if got, want := r.URL.Path, "/v1/responses"; got != want {
			t.Fatalf("path mismatch: got %s want %s", got, want)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}

		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}

		if callCount == 1 {
			if _, ok := payload["previous_response_id"]; ok {
				t.Fatalf("did not expect previous_response_id on first call")
			}
			input := payload["input"].([]any)
			if len(input) != 1 {
				t.Fatalf("expected one initial input item, got %d", len(input))
			}
			io.WriteString(w, `{
				"id":"resp_1",
				"output":[
					{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Let me inspect it."}]},
					{"id":"fc_1","type":"function_call","call_id":"call_1","name":"read_file","arguments":"{\"path\":\"README.md\"}"}
				]
			}`)
			return
		}

		if got := payload["previous_response_id"]; got != "resp_1" {
			t.Fatalf("expected previous_response_id resp_1, got %#v", got)
		}
		input := payload["input"].([]any)
		if len(input) != 1 {
			t.Fatalf("expected one delta input item, got %d", len(input))
		}
		item := input[0].(map[string]any)
		if got := item["type"]; got != "function_call_output" {
			t.Fatalf("expected function_call_output, got %#v", got)
		}
		if got := item["call_id"]; got != "call_1" {
			t.Fatalf("expected call_id call_1, got %#v", got)
		}

		io.WriteString(w, `{
			"id":"resp_2",
			"output":[
				{"type":"message","role":"assistant","content":[{"type":"output_text","text":"README.md exists."}]}
			]
		}`)
	}))
	defer server.Close()

	client := NewOpenAIResponsesClient("test-key", server.URL)

	first, err := client.CreateMessage(context.Background(), Request{
		Model:     "gpt-5.4",
		MaxTokens: 256,
		System:    "You are helpful.",
		Messages: []Message{
			{
				Role: "user",
				Content: []ContentBlock{
					{Type: "text", Text: "check README"},
				},
			},
		},
		Tools: []ToolDefinition{
			{
				Name:        "read_file",
				Description: "Read a file.",
				InputSchema: map[string]any{"type": "object"},
				Strict:      true,
			},
		},
	})
	if err != nil {
		t.Fatalf("first CreateMessage: %v", err)
	}
	if len(first.Content) != 2 || first.Content[1].Type != "tool_use" {
		t.Fatalf("unexpected first response content: %#v", first.Content)
	}
	if first.Content[1].ID != "call_1" {
		t.Fatalf("expected call_1, got %#v", first.Content[1].ID)
	}

	second, err := client.CreateMessage(context.Background(), Request{
		Model:     "gpt-5.4",
		MaxTokens: 256,
		System:    "You are helpful.",
		Messages: []Message{
			{
				Role: "user",
				Content: []ContentBlock{
					{Type: "text", Text: "check README"},
				},
			},
			{
				Role:    "assistant",
				Content: first.Content,
			},
			{
				Role: "user",
				Content: []ContentBlock{
					{Type: "tool_result", ToolUseID: "call_1", Content: "README contents"},
				},
			},
		},
		Tools: []ToolDefinition{
			{
				Name:        "read_file",
				Description: "Read a file.",
				InputSchema: map[string]any{"type": "object"},
				Strict:      true,
			},
		},
	})
	if err != nil {
		t.Fatalf("second CreateMessage: %v", err)
	}
	if got := strings.TrimSpace(second.Content[0].Text); got != "README.md exists." {
		t.Fatalf("unexpected final text %q", got)
	}
}
