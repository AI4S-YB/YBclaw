package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

type Definition struct {
	Name        string
	Description string
	InputSchema map[string]any
	Strict      bool
}

type Result struct {
	Content string
	IsError bool
}

type Env struct {
	Workspace string
}

type Tool interface {
	Definition() Definition
	Call(ctx context.Context, env Env, input json.RawMessage) (Result, error)
}

type Registry struct {
	ordered []Tool
	byName  map[string]Tool
}

func NewRegistry(toolList ...Tool) *Registry {
	registry := &Registry{
		ordered: make([]Tool, 0, len(toolList)),
		byName:  make(map[string]Tool, len(toolList)),
	}
	for _, tool := range toolList {
		name := tool.Definition().Name
		if _, exists := registry.byName[name]; exists {
			panic(fmt.Sprintf("duplicate tool registered: %s", name))
		}
		registry.ordered = append(registry.ordered, tool)
		registry.byName[name] = tool
	}
	return registry
}

func (r *Registry) Definitions() []Definition {
	definitions := make([]Definition, 0, len(r.ordered))
	for _, tool := range r.ordered {
		definitions = append(definitions, tool.Definition())
	}
	return definitions
}

func (r *Registry) Execute(ctx context.Context, env Env, name string, input json.RawMessage) Result {
	tool, ok := r.byName[name]
	if !ok {
		return Result{
			Content: fmt.Sprintf("unknown tool: %s", name),
			IsError: true,
		}
	}

	result, err := tool.Call(ctx, env, input)
	if err != nil {
		return Result{
			Content: err.Error(),
			IsError: true,
		}
	}
	return result
}
