package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"claw/internal/agent"
	"claw/internal/model"
	"claw/internal/tools"
)

func main() {
	var (
		prompt     = flag.String("prompt", "", "user prompt; if empty, reads stdin or trailing args")
		workdir    = flag.String("workdir", ".", "workspace root")
		provider   = flag.String("provider", envOrDefault("CLAW_PROVIDER", "anthropic"), "model provider: anthropic, openai-chat, or openai-responses")
		modelName  = flag.String("model", "", "model name; defaults depend on -provider")
		maxTurns   = flag.Int("max-turns", 12, "maximum tool-use iterations")
		maxTokens  = flag.Int("max-tokens", 4096, "max output tokens per model call")
		baseURL    = flag.String("base-url", "", "provider base URL; defaults depend on -provider")
		apiKey     = flag.String("api-key", "", "provider API key; defaults depend on -provider")
		quietTools = flag.Bool("quiet-tools", false, "hide tool logs on stderr")
	)
	flag.Parse()

	userPrompt, err := resolvePrompt(*prompt, flag.Args(), os.Stdin)
	if err != nil {
		fatal(err)
	}
	if userPrompt == "" {
		fatal(fmt.Errorf("missing prompt"))
	}
	workspace, err := filepath.Abs(*workdir)
	if err != nil {
		fatal(fmt.Errorf("resolve workdir: %w", err))
	}

	providerValue, err := model.ParseProvider(*provider)
	if err != nil {
		fatal(err)
	}

	modelValue := strings.TrimSpace(*modelName)
	if modelValue == "" {
		modelValue = envOrDefault("CLAW_MODEL", providerValue.DefaultModel())
	}

	apiKeyValue := strings.TrimSpace(*apiKey)
	if apiKeyValue == "" {
		apiKeyValue = strings.TrimSpace(os.Getenv(providerValue.APIKeyEnvVar()))
	}
	if apiKeyValue == "" {
		fatal(fmt.Errorf("missing API key; set %s or use -api-key", providerValue.APIKeyEnvVar()))
	}

	baseURLValue := strings.TrimSpace(*baseURL)
	if baseURLValue == "" {
		baseURLValue = envOrDefault(providerValue.BaseURLEnvVar(), providerValue.DefaultBaseURL())
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, err := model.NewClient(providerValue, apiKeyValue, baseURLValue)
	if err != nil {
		fatal(err)
	}
	registry := tools.DefaultRegistry()

	claw := agent.Agent{
		Client:    client,
		Tools:     registry,
		Workspace: workspace,
		Model:     modelValue,
		MaxTurns:  *maxTurns,
		MaxTokens: *maxTokens,
	}

	if !*quietTools {
		claw.Callbacks = agent.Callbacks{
			OnToolCall: func(name string, input json.RawMessage) {
				fmt.Fprintf(os.Stderr, "[tool] %s %s\n", name, compactJSON(input))
			},
			OnToolResult: func(name string, result tools.Result) {
				status := "ok"
				if result.IsError {
					status = "error"
				}
				fmt.Fprintf(os.Stderr, "[tool-result] %s %s\n", name, status)
			},
		}
	}

	result, err := claw.Run(ctx, userPrompt)
	if err != nil {
		fatal(err)
	}

	if result.Answer != "" {
		fmt.Println(result.Answer)
	}
}

func resolvePrompt(flagPrompt string, args []string, stdin io.Reader) (string, error) {
	if strings.TrimSpace(flagPrompt) != "" {
		return strings.TrimSpace(flagPrompt), nil
	}
	if len(args) > 0 {
		return strings.TrimSpace(strings.Join(args, " ")), nil
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func compactJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	var dst bytes.Buffer
	if err := json.Compact(&dst, raw); err == nil {
		return dst.String()
	}
	return string(raw)
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
