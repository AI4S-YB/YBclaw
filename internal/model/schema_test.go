package model

import "testing"

func TestNormalizedToolDefinitionsClosesObjectSchemas(t *testing.T) {
	definitions := []ToolDefinition{
		{
			Name:        "demo",
			Description: "demo tool",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type": "string",
					},
					"options": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"recursive": map[string]any{
								"type": "boolean",
							},
						},
					},
					"entries": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"name": map[string]any{
									"type": "string",
								},
							},
						},
					},
				},
			},
		},
	}

	got := normalizedToolDefinitions(definitions)
	root := got[0].InputSchema
	if root["additionalProperties"] != false {
		t.Fatalf("root object missing additionalProperties=false: %#v", root)
	}

	props := root["properties"].(map[string]any)
	options := props["options"].(map[string]any)
	if options["additionalProperties"] != false {
		t.Fatalf("nested object missing additionalProperties=false: %#v", options)
	}

	entries := props["entries"].(map[string]any)
	itemSchema := entries["items"].(map[string]any)
	if itemSchema["additionalProperties"] != false {
		t.Fatalf("array item object missing additionalProperties=false: %#v", itemSchema)
	}
}

func TestNormalizedOpenAIToolDefinitionsRequireAllProperties(t *testing.T) {
	definitions := []ToolDefinition{
		{
			Name:        "list_files",
			Description: "list files",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type": "string",
					},
					"recursive": map[string]any{
						"type": "boolean",
					},
					"max_results": map[string]any{
						"type": "integer",
					},
				},
			},
		},
	}

	got := normalizedOpenAIToolDefinitions(definitions)
	root := got[0].InputSchema
	required := parseRequiredKeys(root["required"])
	if len(required) != 3 {
		t.Fatalf("expected all properties in required, got %#v", root["required"])
	}

	props := root["properties"].(map[string]any)
	pathSchema := props["path"].(map[string]any)
	if !schemaAllowsNull(pathSchema) {
		t.Fatalf("expected optional property to become nullable, got %#v", pathSchema)
	}
	recursiveSchema := props["recursive"].(map[string]any)
	if !schemaAllowsNull(recursiveSchema) {
		t.Fatalf("expected optional property to become nullable, got %#v", recursiveSchema)
	}
}

func TestNormalizedOpenAIToolDefinitionsPreserveRequiredFields(t *testing.T) {
	definitions := []ToolDefinition{
		{
			Name:        "read_file",
			Description: "read file",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type": "string",
					},
					"max_lines": map[string]any{
						"type": "integer",
					},
				},
				"required": []string{"path"},
			},
		},
	}

	got := normalizedOpenAIToolDefinitions(definitions)
	root := got[0].InputSchema
	required := parseRequiredKeys(root["required"])
	if len(required) != 2 {
		t.Fatalf("expected all properties in required, got %#v", root["required"])
	}

	props := root["properties"].(map[string]any)
	pathSchema := props["path"].(map[string]any)
	if schemaAllowsNull(pathSchema) {
		t.Fatalf("expected originally required field to stay non-nullable, got %#v", pathSchema)
	}

	maxLinesSchema := props["max_lines"].(map[string]any)
	if !schemaAllowsNull(maxLinesSchema) {
		t.Fatalf("expected optional field to become nullable, got %#v", maxLinesSchema)
	}
}
