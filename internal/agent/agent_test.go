package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claw/internal/model"
	"claw/internal/tools"
)

type fakeClient struct {
	requests  []model.Request
	responses []model.Response
}

func (f *fakeClient) CreateMessage(_ context.Context, req model.Request) (model.Response, error) {
	f.requests = append(f.requests, req)
	if len(f.responses) == 0 {
		return model.Response{}, fmt.Errorf("no fake responses left")
	}
	response := f.responses[0]
	f.responses = f.responses[1:]
	return response, nil
}

func TestAgentRunExecutesToolLoop(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	client := &fakeClient{
		responses: []model.Response{
			{
				Content: []model.ContentBlock{
					{Type: "text", Text: "I'll inspect the file."},
					{
						Type:  "tool_use",
						ID:    "tool-1",
						Name:  "read_file",
						Input: json.RawMessage(`{"path":"hello.txt","max_lines":10}`),
					},
				},
			},
			{
				Content: []model.ContentBlock{
					{Type: "text", Text: "The file contains two lines: one and two."},
				},
			},
		},
	}

	claw := Agent{
		Client:    client,
		Tools:     tools.DefaultRegistry(),
		Workspace: dir,
		Model:     "test-model",
		MaxTurns:  4,
	}

	result, err := claw.Run(context.Background(), "what is inside hello.txt?")
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}
	if got, want := result.Answer, "The file contains two lines: one and two."; got != want {
		t.Fatalf("answer mismatch: got %q want %q", got, want)
	}
	if len(client.requests) != 2 {
		t.Fatalf("expected 2 model requests, got %d", len(client.requests))
	}
	if len(client.requests[1].Messages) != 3 {
		t.Fatalf("expected second request to contain 3 messages, got %d", len(client.requests[1].Messages))
	}

	lastMessage := client.requests[1].Messages[2]
	if lastMessage.Role != "user" {
		t.Fatalf("expected tool_result message to be user role, got %q", lastMessage.Role)
	}
	if len(lastMessage.Content) != 1 || lastMessage.Content[0].Type != "tool_result" {
		t.Fatalf("expected final message to contain one tool_result block, got %#v", lastMessage.Content)
	}
	content, ok := lastMessage.Content[0].Content.(string)
	if !ok {
		t.Fatalf("expected tool_result content to be string, got %#v", lastMessage.Content[0].Content)
	}
	if !strings.Contains(content, "hello.txt") {
		t.Fatalf("tool result missing filename: %q", content)
	}
}

func TestAgentStopsAtMaxTurns(t *testing.T) {
	client := &fakeClient{
		responses: []model.Response{
			{
				Content: []model.ContentBlock{
					{
						Type:  "tool_use",
						ID:    "tool-1",
						Name:  "list_files",
						Input: json.RawMessage(`{"path":"."}`),
					},
				},
			},
		},
	}

	claw := Agent{
		Client:    client,
		Tools:     tools.DefaultRegistry(),
		Workspace: t.TempDir(),
		Model:     "test-model",
		MaxTurns:  1,
	}

	_, err := claw.Run(context.Background(), "list files")
	if err == nil || !strings.Contains(err.Error(), "max turns exceeded") {
		t.Fatalf("expected max-turns error, got %v", err)
	}
}
