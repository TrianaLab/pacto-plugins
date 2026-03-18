package infer

import (
	"strings"
	"testing"
)

func TestBuildSpecMinimal(t *testing.T) {
	result := &Result{
		Info: AppInfo{Title: "Test", Version: "1.0.0"},
		Routes: []Route{
			{Method: "GET", Path: "/health"},
		},
	}

	spec, err := BuildSpec(result)
	if err != nil {
		t.Fatalf("BuildSpec error: %v", err)
	}

	assertContains(t, spec, "openapi: 3.1.0")
	assertContains(t, spec, "title: Test")
	assertContains(t, spec, "version: 1.0.0")
	assertContains(t, spec, "/health:")
	assertContains(t, spec, "get:")
}

func TestBuildSpecWithParams(t *testing.T) {
	result := &Result{
		Routes: []Route{
			{
				Method: "GET", Path: "/users/{id}",
				Params: []Param{
					{Name: "id", In: "path", Type: "integer", Required: true},
				},
				ResBody: &BodyRef{Name: "User"},
			},
		},
		Schemas: []Schema{
			{Name: "User", Fields: []Field{
				{Name: "id", Type: "integer"},
				{Name: "name", Type: "string"},
			}},
		},
	}

	spec, err := BuildSpec(result)
	if err != nil {
		t.Fatalf("BuildSpec error: %v", err)
	}

	assertContains(t, spec, "name: id")
	assertContains(t, spec, "in: path")
	assertContains(t, spec, "required: true")
	assertContains(t, spec, "$ref: '#/components/schemas/User'")
	assertContains(t, spec, "type: object")
	assertContains(t, spec, "type: integer")
	assertContains(t, spec, "type: string")
}

func TestBuildSpecWithArrayBody(t *testing.T) {
	result := &Result{
		Routes: []Route{
			{
				Method:  "GET",
				Path:    "/items",
				ResBody: &BodyRef{Name: "Item", IsArray: true},
			},
		},
	}

	spec, err := BuildSpec(result)
	if err != nil {
		t.Fatalf("BuildSpec error: %v", err)
	}

	assertContains(t, spec, "type: array")
	assertContains(t, spec, "$ref: '#/components/schemas/Item'")
}

func TestBuildSpecWithRequestBody(t *testing.T) {
	result := &Result{
		Routes: []Route{
			{
				Method:  "POST",
				Path:    "/users",
				ReqBody: &BodyRef{Name: "UserCreate"},
			},
		},
	}

	spec, err := BuildSpec(result)
	if err != nil {
		t.Fatalf("BuildSpec error: %v", err)
	}

	assertContains(t, spec, "requestBody:")
	assertContains(t, spec, "required: true")
	assertContains(t, spec, "application/json:")
	assertContains(t, spec, "$ref: '#/components/schemas/UserCreate'")
}

func TestBuildSpecDefaults(t *testing.T) {
	result := &Result{
		Routes: []Route{{Method: "GET", Path: "/"}},
	}

	spec, err := BuildSpec(result)
	if err != nil {
		t.Fatalf("BuildSpec error: %v", err)
	}

	assertContains(t, spec, "title: API")
	assertContains(t, spec, "version: 0.0.0")
}

func TestBuildSpecNoRoutes(t *testing.T) {
	result := &Result{}
	_, err := BuildSpec(result)
	if err == nil {
		t.Error("expected error for empty routes")
	}
}

func TestBuildSpecMultipleMethods(t *testing.T) {
	result := &Result{
		Routes: []Route{
			{Method: "GET", Path: "/users"},
			{Method: "POST", Path: "/users"},
		},
	}

	spec, err := BuildSpec(result)
	if err != nil {
		t.Fatalf("BuildSpec error: %v", err)
	}

	assertContains(t, spec, "get:")
	assertContains(t, spec, "post:")
}

func TestBuildSpecJSONFormat(t *testing.T) {
	result := &Result{
		Routes: []Route{{Method: "GET", Path: "/health"}},
	}
	spec, err := BuildSpec(result, FormatJSON)
	if err != nil {
		t.Fatalf("BuildSpec error: %v", err)
	}
	if !strings.Contains(spec, "\"openapi\": \"3.1.0\"") {
		t.Errorf("expected JSON format, got:\n%s", spec)
	}
}

func TestBuildSpecWithSummary(t *testing.T) {
	result := &Result{
		Routes: []Route{
			{Method: "GET", Path: "/items", Summary: "List items"},
		},
	}
	spec, err := BuildSpec(result)
	if err != nil {
		t.Fatalf("BuildSpec error: %v", err)
	}
	assertContains(t, spec, "summary: List items")
}

func TestBuildSpecArraySchemaField(t *testing.T) {
	result := &Result{
		Routes: []Route{{Method: "GET", Path: "/"}},
		Schemas: []Schema{
			{Name: "Owner", Fields: []Field{
				{Name: "tags", Type: "string", IsArray: true},
			}},
		},
	}
	spec, err := BuildSpec(result)
	if err != nil {
		t.Fatalf("BuildSpec error: %v", err)
	}
	assertContains(t, spec, "type: array")
	assertContains(t, spec, "type: string")
}

func TestBuildSpecRequestBodyArray(t *testing.T) {
	result := &Result{
		Routes: []Route{
			{
				Method:  "POST",
				Path:    "/batch",
				ReqBody: &BodyRef{Name: "Item", IsArray: true},
			},
		},
	}
	spec, err := BuildSpec(result)
	if err != nil {
		t.Fatalf("BuildSpec error: %v", err)
	}
	assertContains(t, spec, "type: array")
	assertContains(t, spec, "$ref: '#/components/schemas/Item'")
}

func TestBuildSpecPrimitiveResponseBody(t *testing.T) {
	result := &Result{
		Routes: []Route{
			{
				Method:  "GET",
				Path:    "/count",
				ResBody: &BodyRef{Name: "integer"},
			},
		},
	}
	spec, err := BuildSpec(result)
	if err != nil {
		t.Fatalf("BuildSpec error: %v", err)
	}
	assertContains(t, spec, "type: integer")
}

func TestMarshalYAML(t *testing.T) {
	out, err := marshalYAML(map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("marshalYAML error: %v", err)
	}
	assertContains(t, out, "key: value")
}

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected output to contain %q\ngot:\n%s", substr, s)
	}
}
