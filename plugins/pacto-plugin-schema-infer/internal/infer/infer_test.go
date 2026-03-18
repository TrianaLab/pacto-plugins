package infer

import (
	"encoding/json"
	"testing"
)

func TestInferValue(t *testing.T) {
	tests := []struct {
		name string
		val  any
		want string
	}{
		{name: "string", val: "hello", want: `{"type":"string"}`},
		{name: "float64", val: 3.14, want: `{"type":"number"}`},
		{name: "int64", val: int64(42), want: `{"type":"integer"}`},
		{name: "int", val: 10, want: `{"type":"integer"}`},
		{name: "bool", val: true, want: `{"type":"boolean"}`},
		{name: "nil", val: nil, want: `{"type":"null"}`},
		{name: "unknown type", val: uint8(1), want: `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferValue(tt.val)
			assertJSON(t, got, tt.want)
		})
	}
}

func TestInferObject(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
		want string
	}{
		{
			name: "flat object",
			data: map[string]any{"name": "app", "port": float64(8080)},
			want: `{"type":"object","properties":{"name":{"type":"string"},"port":{"type":"number"}},"required":["name","port"]}`,
		},
		{
			name: "nested object",
			data: map[string]any{
				"db": map[string]any{
					"host": "localhost",
					"port": int64(5432),
				},
			},
			want: `{"type":"object","properties":{"db":{"type":"object","properties":{"host":{"type":"string"},"port":{"type":"integer"}},"required":["host","port"]}},"required":["db"]}`,
		},
		{
			name: "empty object",
			data: map[string]any{},
			want: `{"type":"object","properties":{},"required":[]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferObject(tt.data)
			assertJSON(t, got, tt.want)
		})
	}
}

func TestInferArray(t *testing.T) {
	tests := []struct {
		name string
		data []any
		want string
	}{
		{
			name: "string array",
			data: []any{"a", "b"},
			want: `{"type":"array","items":{"type":"string"}}`,
		},
		{
			name: "number array",
			data: []any{float64(1), float64(2)},
			want: `{"type":"array","items":{"type":"number"}}`,
		},
		{
			name: "object array",
			data: []any{map[string]any{"id": float64(1)}},
			want: `{"type":"array","items":{"type":"object","properties":{"id":{"type":"number"}},"required":["id"]}}`,
		},
		{
			name: "empty array",
			data: []any{},
			want: `{"type":"array","items":{}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferArray(tt.data)
			assertJSON(t, got, tt.want)
		})
	}
}

func testSchemaData() map[string]any {
	return map[string]any{
		"app": map[string]any{
			"name":  "my-service",
			"debug": true,
		},
		"database": map[string]any{
			"host":     "localhost",
			"port":     int64(5432),
			"replicas": []any{map[string]any{"host": "r1", "port": int64(5433)}},
		},
		"features": []any{"auth", "logging"},
		"timeout":  float64(30),
		"nothing":  nil,
	}
}

func TestSchemaMetadata(t *testing.T) {
	schema := Schema(testSchemaData())

	if schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
		t.Errorf("$schema = %v, want draft/2020-12", schema["$schema"])
	}
	if schema["additionalProperties"] != false {
		t.Errorf("additionalProperties = %v, want false", schema["additionalProperties"])
	}
	if schema["type"] != "object" {
		t.Errorf("type = %v, want object", schema["type"])
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties is not a map")
	}

	for _, key := range []string{"app", "database", "features", "timeout", "nothing"} {
		if _, exists := props[key]; !exists {
			t.Errorf("missing property %q", key)
		}
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("required is not a string slice")
	}
	expectedRequired := []string{"app", "database", "features", "nothing", "timeout"}
	if len(required) != len(expectedRequired) {
		t.Fatalf("required length = %d, want %d", len(required), len(expectedRequired))
	}
	for i, r := range required {
		if r != expectedRequired[i] {
			t.Errorf("required[%d] = %q, want %q", i, r, expectedRequired[i])
		}
	}
}

func TestSchemaTypeInference(t *testing.T) {
	schema := Schema(testSchemaData())
	props := schema["properties"].(map[string]any)

	appProps := props["app"].(map[string]any)["properties"].(map[string]any)
	if appProps["name"].(map[string]any)["type"] != "string" {
		t.Error("app.name should be string")
	}
	if appProps["debug"].(map[string]any)["type"] != "boolean" {
		t.Error("app.debug should be boolean")
	}

	dbProps := props["database"].(map[string]any)["properties"].(map[string]any)
	replicas := dbProps["replicas"].(map[string]any)
	if replicas["type"] != "array" {
		t.Error("database.replicas should be array")
	}

	if props["timeout"].(map[string]any)["type"] != "number" {
		t.Error("timeout should be number")
	}
	if props["nothing"].(map[string]any)["type"] != "null" {
		t.Error("nothing should be null")
	}
	if props["features"].(map[string]any)["type"] != "array" {
		t.Error("features should be array")
	}
}

func TestSchemaIntegration(t *testing.T) {
	// Simulates a realistic config parsed from JSON (all numbers are float64)
	data := map[string]any{
		"server": map[string]any{
			"host":     "0.0.0.0",
			"port":     float64(8080),
			"tls":      true,
			"timeouts": map[string]any{"read": float64(30), "write": float64(60)},
		},
		"logging": map[string]any{
			"level":   "info",
			"outputs": []any{"stdout", "file"},
		},
		"cache": map[string]any{
			"enabled": true,
			"ttl":     float64(300),
			"backends": []any{
				map[string]any{"type": "redis", "host": "localhost"},
			},
		},
	}

	schema := Schema(data)

	// Marshal and unmarshal to verify it's valid JSON
	b, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal schema: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}

	// Verify structure
	if parsed["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
		t.Error("missing $schema")
	}
	if parsed["type"] != "object" {
		t.Error("root type should be object")
	}

	props := parsed["properties"].(map[string]any)
	serverProps := props["server"].(map[string]any)["properties"].(map[string]any)
	timeouts := serverProps["timeouts"].(map[string]any)["properties"].(map[string]any)
	if timeouts["read"].(map[string]any)["type"] != "number" {
		t.Error("server.timeouts.read should be number")
	}

	cacheProps := props["cache"].(map[string]any)["properties"].(map[string]any)
	backends := cacheProps["backends"].(map[string]any)
	if backends["type"] != "array" {
		t.Error("cache.backends should be array")
	}
	items := backends["items"].(map[string]any)
	if items["type"] != "object" {
		t.Error("cache.backends items should be object")
	}
}

func assertJSON(t *testing.T, got map[string]any, wantJSON string) {
	t.Helper()
	gotBytes, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("failed to marshal got: %v", err)
	}
	// Normalize by unmarshalling both
	var gotNorm, wantNorm any
	if err := json.Unmarshal(gotBytes, &gotNorm); err != nil {
		t.Fatalf("failed to unmarshal got: %v", err)
	}
	if err := json.Unmarshal([]byte(wantJSON), &wantNorm); err != nil {
		t.Fatalf("failed to unmarshal want: %v", err)
	}
	gotNormBytes, _ := json.Marshal(gotNorm)
	wantNormBytes, _ := json.Marshal(wantNorm)
	if string(gotNormBytes) != string(wantNormBytes) {
		t.Errorf("got  %s\nwant %s", gotNormBytes, wantNormBytes)
	}
}
