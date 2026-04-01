package model

import (
	"maps"
	"sort"
)

func normalizedToolDefinitions(definitions []ToolDefinition) []ToolDefinition {
	if len(definitions) == 0 {
		return nil
	}

	out := make([]ToolDefinition, 0, len(definitions))
	for _, definition := range definitions {
		clone := ToolDefinition{
			Name:        definition.Name,
			Description: definition.Description,
			Strict:      definition.Strict,
		}
		if definition.InputSchema != nil {
			clone.InputSchema = normalizeJSONSchema(definition.InputSchema)
		}
		out = append(out, clone)
	}
	return out
}

func normalizedOpenAIToolDefinitions(definitions []ToolDefinition) []ToolDefinition {
	if len(definitions) == 0 {
		return nil
	}

	out := make([]ToolDefinition, 0, len(definitions))
	for _, definition := range definitions {
		clone := ToolDefinition{
			Name:        definition.Name,
			Description: definition.Description,
			Strict:      definition.Strict,
		}
		if definition.InputSchema != nil {
			clone.InputSchema = normalizeOpenAIJSONSchema(definition.InputSchema)
		}
		out = append(out, clone)
	}
	return out
}

func normalizeJSONSchema(schema map[string]any) map[string]any {
	cloned := maps.Clone(schema)

	if schemaType, ok := cloned["type"].(string); ok && schemaType == "object" {
		if _, exists := cloned["additionalProperties"]; !exists {
			cloned["additionalProperties"] = false
		}
	}

	if properties, ok := cloned["properties"].(map[string]any); ok {
		next := make(map[string]any, len(properties))
		for key, value := range properties {
			if child, ok := value.(map[string]any); ok {
				next[key] = normalizeJSONSchema(child)
			} else {
				next[key] = value
			}
		}
		cloned["properties"] = next
	}

	if items, ok := cloned["items"].(map[string]any); ok {
		cloned["items"] = normalizeJSONSchema(items)
	}

	if anyOf, ok := cloned["anyOf"].([]any); ok {
		cloned["anyOf"] = normalizeSchemaSlice(anyOf)
	}
	if oneOf, ok := cloned["oneOf"].([]any); ok {
		cloned["oneOf"] = normalizeSchemaSlice(oneOf)
	}
	if allOf, ok := cloned["allOf"].([]any); ok {
		cloned["allOf"] = normalizeSchemaSlice(allOf)
	}

	return cloned
}

func normalizeOpenAIJSONSchema(schema map[string]any) map[string]any {
	base := normalizeJSONSchema(schema)
	return normalizeOpenAIJSONSchemaRecursive(base)
}

func normalizeOpenAIJSONSchemaRecursive(schema map[string]any) map[string]any {
	cloned := maps.Clone(schema)

	properties, hasProperties := cloned["properties"].(map[string]any)
	if hasProperties {
		requiredSet := map[string]struct{}{}
		for _, key := range parseRequiredKeys(cloned["required"]) {
			requiredSet[key] = struct{}{}
		}

		keys := make([]string, 0, len(properties))
		nextProps := make(map[string]any, len(properties))
		for key, value := range properties {
			keys = append(keys, key)

			child, isMap := value.(map[string]any)
			if !isMap {
				nextProps[key] = value
				continue
			}

			normalizedChild := normalizeOpenAIJSONSchemaRecursive(child)
			if _, required := requiredSet[key]; !required {
				normalizedChild = wrapSchemaAsNullable(normalizedChild)
			}
			nextProps[key] = normalizedChild
		}
		sort.Strings(keys)
		cloned["properties"] = nextProps
		cloned["required"] = keys
	} else if _, ok := cloned["required"]; ok {
		cloned["required"] = parseRequiredKeys(cloned["required"])
	}

	if items, ok := cloned["items"].(map[string]any); ok {
		cloned["items"] = normalizeOpenAIJSONSchemaRecursive(items)
	}
	if anyOf, ok := cloned["anyOf"].([]any); ok {
		cloned["anyOf"] = normalizeOpenAISchemaSlice(anyOf)
	}
	if oneOf, ok := cloned["oneOf"].([]any); ok {
		cloned["oneOf"] = normalizeOpenAISchemaSlice(oneOf)
	}
	if allOf, ok := cloned["allOf"].([]any); ok {
		cloned["allOf"] = normalizeOpenAISchemaSlice(allOf)
	}

	return cloned
}

func normalizeSchemaSlice(items []any) []any {
	next := make([]any, 0, len(items))
	for _, item := range items {
		if child, ok := item.(map[string]any); ok {
			next = append(next, normalizeJSONSchema(child))
		} else {
			next = append(next, item)
		}
	}
	return next
}

func normalizeOpenAISchemaSlice(items []any) []any {
	next := make([]any, 0, len(items))
	for _, item := range items {
		if child, ok := item.(map[string]any); ok {
			next = append(next, normalizeOpenAIJSONSchemaRecursive(child))
		} else {
			next = append(next, item)
		}
	}
	return next
}

func parseRequiredKeys(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string{}, typed...)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if key, ok := item.(string); ok {
				out = append(out, key)
			}
		}
		return out
	default:
		return nil
	}
}

func wrapSchemaAsNullable(schema map[string]any) map[string]any {
	if schemaAllowsNull(schema) {
		return schema
	}

	cloned := maps.Clone(schema)
	if schemaType, ok := cloned["type"].(string); ok && schemaType != "" {
		delete(cloned, "type")
		cloned["anyOf"] = []any{
			map[string]any{"type": schemaType},
			map[string]any{"type": "null"},
		}
		return cloned
	}

	if schemaTypes, ok := cloned["type"].([]any); ok {
		next := append([]any{}, schemaTypes...)
		next = append(next, "null")
		cloned["type"] = next
		return cloned
	}

	if anyOf, ok := cloned["anyOf"].([]any); ok {
		next := append([]any{}, anyOf...)
		next = append(next, map[string]any{"type": "null"})
		cloned["anyOf"] = next
		return cloned
	}

	cloned["anyOf"] = []any{
		cloned,
		map[string]any{"type": "null"},
	}
	return cloned
}

func schemaAllowsNull(schema map[string]any) bool {
	if schemaType, ok := schema["type"].(string); ok {
		return schemaType == "null"
	}
	if schemaTypes, ok := schema["type"].([]any); ok {
		for _, item := range schemaTypes {
			if s, ok := item.(string); ok && s == "null" {
				return true
			}
		}
	}
	if anyOf, ok := schema["anyOf"].([]any); ok {
		for _, item := range anyOf {
			child, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if schemaType, ok := child["type"].(string); ok && schemaType == "null" {
				return true
			}
		}
	}
	return false
}
