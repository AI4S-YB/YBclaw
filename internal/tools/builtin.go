package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	defaultReadMaxLines   = 200
	maxReadMaxLines       = 500
	defaultListMaxResults = 200
	maxListMaxResults     = 1000
	defaultCommandTimeout = 30 * time.Second
	maxCommandTimeout     = 5 * time.Minute
	maxCommandOutputBytes = 20_000
)

func DefaultRegistry() *Registry {
	return NewRegistry(
		ListFilesTool{},
		ReadFileTool{},
		WriteFileTool{},
		RunCommandTool{},
	)
}

type ListFilesTool struct{}
type ReadFileTool struct{}
type WriteFileTool struct{}
type RunCommandTool struct{}

func (ListFilesTool) Definition() Definition {
	return Definition{
		Name:        "list_files",
		Description: "List files or directories inside the workspace.",
		Strict:      true,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Directory or file path relative to the workspace. Defaults to '.'.",
				},
				"recursive": map[string]any{
					"type":        "boolean",
					"description": "When true, walk subdirectories recursively.",
				},
				"max_results": map[string]any{
					"type":        "integer",
					"description": "Maximum number of entries to return. Defaults to 200.",
				},
			},
		},
	}
}

func (ReadFileTool) Definition() Definition {
	return Definition{
		Name:        "read_file",
		Description: "Read a text file from the workspace with optional line windowing.",
		Strict:      true,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to a text file inside the workspace.",
				},
				"start_line": map[string]any{
					"type":        "integer",
					"description": "1-based start line. Defaults to 1.",
				},
				"max_lines": map[string]any{
					"type":        "integer",
					"description": "Maximum number of lines to return. Defaults to 200 and is capped at 500.",
				},
			},
			"required": []string{"path"},
		},
	}
}

func (WriteFileTool) Definition() Definition {
	return Definition{
		Name:        "write_file",
		Description: "Create or overwrite a file inside the workspace.",
		Strict:      true,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to write, relative to the workspace.",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Full file content to write.",
				},
			},
			"required": []string{"path", "content"},
		},
	}
}

func (RunCommandTool) Definition() Definition {
	return Definition{
		Name:        "run_command",
		Description: "Run a shell command inside the workspace and capture stdout and stderr.",
		Strict:      true,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "Shell command to execute.",
				},
				"timeout_seconds": map[string]any{
					"type":        "integer",
					"description": "Optional timeout in seconds. Defaults to 30, max 300.",
				},
			},
			"required": []string{"command"},
		},
	}
}

func (ListFilesTool) Call(_ context.Context, env Env, input json.RawMessage) (Result, error) {
	var args struct {
		Path       string `json:"path"`
		Recursive  bool   `json:"recursive"`
		MaxResults int    `json:"max_results"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return Result{}, fmt.Errorf("parse list_files input: %w", err)
	}

	target, displayPath, err := resolvePath(env.Workspace, defaultIfEmpty(args.Path, "."), true)
	if err != nil {
		return Result{}, err
	}

	info, err := os.Stat(target)
	if err != nil {
		return Result{}, fmt.Errorf("stat %s: %w", displayPath, err)
	}

	limit := clampPositive(args.MaxResults, defaultListMaxResults, maxListMaxResults)
	items := make([]string, 0, limit)

	if !info.IsDir() {
		return Result{Content: displayPath}, nil
	}

	if args.Recursive {
		err = filepath.WalkDir(target, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if path == target {
				return nil
			}
			rel, relErr := filepath.Rel(target, path)
			if relErr != nil {
				return relErr
			}
			item := rel
			if d.IsDir() {
				item += string(filepath.Separator)
			}
			items = append(items, item)
			if len(items) >= limit {
				return fs.SkipAll
			}
			return nil
		})
		if err != nil && !errors.Is(err, fs.SkipAll) {
			return Result{}, fmt.Errorf("walk %s: %w", displayPath, err)
		}
		sort.Strings(items)
	} else {
		entries, err := os.ReadDir(target)
		if err != nil {
			return Result{}, fmt.Errorf("read dir %s: %w", displayPath, err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() {
				name += string(filepath.Separator)
			}
			items = append(items, name)
			if len(items) >= limit {
				break
			}
		}
		sort.Strings(items)
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "path: %s\n", displayPath)
	fmt.Fprintf(&builder, "entries: %d", len(items))
	if len(items) == limit {
		builder.WriteString(" (possibly truncated)")
	}
	builder.WriteString("\n\n")
	for _, item := range items {
		builder.WriteString(item)
		builder.WriteByte('\n')
	}

	return Result{Content: strings.TrimRight(builder.String(), "\n")}, nil
}

func (ReadFileTool) Call(_ context.Context, env Env, input json.RawMessage) (Result, error) {
	var args struct {
		Path      string `json:"path"`
		StartLine int    `json:"start_line"`
		MaxLines  int    `json:"max_lines"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return Result{}, fmt.Errorf("parse read_file input: %w", err)
	}
	if strings.TrimSpace(args.Path) == "" {
		return Result{}, fmt.Errorf("read_file.path is required")
	}

	target, displayPath, err := resolvePath(env.Workspace, args.Path, true)
	if err != nil {
		return Result{}, err
	}

	data, err := os.ReadFile(target)
	if err != nil {
		return Result{}, fmt.Errorf("read %s: %w", displayPath, err)
	}
	if !utf8.Valid(data) {
		return Result{}, fmt.Errorf("%s is not valid UTF-8 text", displayPath)
	}

	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(content, "\n")
	start := args.StartLine
	if start <= 0 {
		start = 1
	}
	maxLines := clampPositive(args.MaxLines, defaultReadMaxLines, maxReadMaxLines)
	if start > len(lines) {
		return Result{
			Content: fmt.Sprintf("path: %s\nerror: start_line %d is beyond file length %d", displayPath, start, len(lines)),
			IsError: true,
		}, nil
	}
	end := start - 1 + maxLines
	if end > len(lines) {
		end = len(lines)
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "path: %s\n", displayPath)
	fmt.Fprintf(&builder, "lines: %d-%d of %d\n\n", start, end, len(lines))
	for i := start - 1; i < end; i++ {
		fmt.Fprintf(&builder, "%d\t%s\n", i+1, lines[i])
	}
	if end < len(lines) {
		fmt.Fprintf(&builder, "\n[truncated: continue with start_line=%d]\n", end+1)
	}

	return Result{Content: strings.TrimRight(builder.String(), "\n")}, nil
}

func (WriteFileTool) Call(_ context.Context, env Env, input json.RawMessage) (Result, error) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return Result{}, fmt.Errorf("parse write_file input: %w", err)
	}
	if strings.TrimSpace(args.Path) == "" {
		return Result{}, fmt.Errorf("write_file.path is required")
	}

	target, displayPath, err := resolvePath(env.Workspace, args.Path, false)
	if err != nil {
		return Result{}, err
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return Result{}, fmt.Errorf("mkdir for %s: %w", displayPath, err)
	}
	if err := os.WriteFile(target, []byte(args.Content), 0o644); err != nil {
		return Result{}, fmt.Errorf("write %s: %w", displayPath, err)
	}

	return Result{
		Content: fmt.Sprintf("wrote %d bytes to %s", len(args.Content), displayPath),
	}, nil
}

func (RunCommandTool) Call(ctx context.Context, env Env, input json.RawMessage) (Result, error) {
	var args struct {
		Command        string `json:"command"`
		TimeoutSeconds int    `json:"timeout_seconds"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return Result{}, fmt.Errorf("parse run_command input: %w", err)
	}
	if strings.TrimSpace(args.Command) == "" {
		return Result{}, fmt.Errorf("run_command.command is required")
	}

	timeout := defaultCommandTimeout
	if args.TimeoutSeconds > 0 {
		timeout = time.Duration(args.TimeoutSeconds) * time.Second
		if timeout > maxCommandTimeout {
			timeout = maxCommandTimeout
		}
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	shell := os.Getenv("SHELL")
	if strings.TrimSpace(shell) == "" {
		shell = "/bin/sh"
	}

	cmd := exec.CommandContext(runCtx, shell, "-lc", args.Command)
	cmd.Dir = env.Workspace

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return Result{
				Content: formatCommandResult(args.Command, -1, stdout.String(), stderr.String(), true),
				IsError: true,
			}, nil
		} else {
			return Result{}, fmt.Errorf("run command: %w", err)
		}
	}

	return Result{
		Content: formatCommandResult(args.Command, exitCode, stdout.String(), stderr.String(), false),
		IsError: exitCode != 0,
	}, nil
}

func resolvePath(workspace, userPath string, mustExist bool) (absolutePath string, displayPath string, err error) {
	root, err := canonicalPath(workspace)
	if err != nil {
		return "", "", fmt.Errorf("resolve workspace: %w", err)
	}

	candidate := userPath
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, candidate)
	}
	candidate = filepath.Clean(candidate)

	checkPath := candidate
	if mustExist {
		checkPath, err = canonicalPath(candidate)
		if err != nil {
			return "", "", fmt.Errorf("resolve path %s: %w", userPath, err)
		}
	} else {
		parent := filepath.Dir(candidate)
		if parentReal, parentErr := canonicalPath(parent); parentErr == nil {
			checkPath = filepath.Join(parentReal, filepath.Base(candidate))
		}
	}

	rel, err := filepath.Rel(root, checkPath)
	if err != nil {
		return "", "", fmt.Errorf("check workspace boundary: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("path escapes workspace: %s", userPath)
	}

	displayPath = filepath.ToSlash(rel)
	if displayPath == "." {
		displayPath = "."
	}
	return candidate, displayPath, nil
}

func canonicalPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return real, nil
}

func clampPositive(value, defaultValue, maxValue int) int {
	if value <= 0 {
		return defaultValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func defaultIfEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func formatCommandResult(command string, exitCode int, stdout string, stderr string, timedOut bool) string {
	stdout = truncateText(strings.TrimSpace(stdout), maxCommandOutputBytes)
	stderr = truncateText(strings.TrimSpace(stderr), maxCommandOutputBytes)

	var builder strings.Builder
	fmt.Fprintf(&builder, "command: %s\n", command)
	if timedOut {
		builder.WriteString("timeout: true\n")
	} else {
		fmt.Fprintf(&builder, "exit_code: %d\n", exitCode)
	}
	builder.WriteString("\nstdout:\n")
	if stdout == "" {
		builder.WriteString("(empty)\n")
	} else {
		builder.WriteString(stdout)
		builder.WriteByte('\n')
	}
	builder.WriteString("\nstderr:\n")
	if stderr == "" {
		builder.WriteString("(empty)")
	} else {
		builder.WriteString(stderr)
	}
	return strings.TrimRight(builder.String(), "\n")
}

func truncateText(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	head := maxBytes * 3 / 4
	tail := maxBytes - head
	return value[:head] + "\n...[truncated]...\n" + value[len(value)-tail:]
}
