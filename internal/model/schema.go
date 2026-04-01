package model

import "maps"

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
