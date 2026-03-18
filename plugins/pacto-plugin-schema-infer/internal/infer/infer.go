// Package infer provides recursive JSON Schema inference from arbitrary data.
package infer

import "sort"

// Schema infers a JSON Schema from a map[string]any data structure.
// It returns a root schema object with $schema, additionalProperties, and
// recursively inferred properties.
func Schema(data map[string]any) map[string]any {
	schema := inferObject(data)
	schema["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	schema["additionalProperties"] = false
	return schema
}

func inferValue(v any) map[string]any {
	switch val := v.(type) {
	case string:
		return map[string]any{"type": "string"}
	case float64:
		return map[string]any{"type": "number"}
	case int64:
		return map[string]any{"type": "integer"}
	case int:
		return map[string]any{"type": "integer"}
	case bool:
		return map[string]any{"type": "boolean"}
	case nil:
		return map[string]any{"type": "null"}
	case map[string]any:
		return inferObject(val)
	case []any:
		return inferArray(val)
	default:
		return map[string]any{}
	}
}

func inferObject(data map[string]any) map[string]any {
	properties := make(map[string]any, len(data))
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		properties[k] = inferValue(data[k])
	}

	return map[string]any{
		"type":       "object",
		"properties": properties,
		"required":   keys,
	}
}

func inferArray(data []any) map[string]any {
	if len(data) == 0 {
		return map[string]any{
			"type":  "array",
			"items": map[string]any{},
		}
	}
	return map[string]any{
		"type":  "array",
		"items": inferValue(data[0]),
	}
}
