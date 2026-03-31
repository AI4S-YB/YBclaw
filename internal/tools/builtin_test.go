package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteAndReadFileRoundTrip(t *testing.T) {
	registry := DefaultRegistry()
	env := Env{Workspace: t.TempDir()}

	writeResult := registry.Execute(context.Background(), env, "write_file", json.RawMessage(`{"path":"notes/todo.txt","content":"a\nb\nc\n"}`))
	if writeResult.IsError {
		t.Fatalf("write_file failed: %s", writeResult.Content)
	}

	readResult := registry.Execute(context.Background(), env, "read_file", json.RawMessage(`{"path":"notes/todo.txt","start_line":2,"max_lines":2}`))
	if readResult.IsError {
		t.Fatalf("read_file failed: %s", readResult.Content)
	}
	if !strings.Contains(readResult.Content, "2\tb") || !strings.Contains(readResult.Content, "3\tc") {
		t.Fatalf("unexpected read output:\n%s", readResult.Content)
	}
}

func TestReadFileRejectsWorkspaceEscape(t *testing.T) {
	env := Env{Workspace: t.TempDir()}
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}

	registry := DefaultRegistry()
	input := json.RawMessage(fmt.Sprintf(`{"path":%q}`, outsideFile))
	result := registry.Execute(context.Background(), env, "read_file", input)
	if !result.IsError {
		t.Fatalf("expected workspace escape to fail, got: %s", result.Content)
	}
	if !strings.Contains(result.Content, "path escapes workspace") {
		t.Fatalf("unexpected error: %s", result.Content)
	}
}
