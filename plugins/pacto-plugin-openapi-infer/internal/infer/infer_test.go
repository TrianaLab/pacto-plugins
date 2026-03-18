package infer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupProject creates a temp directory with the given files.
func setupProject(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for path, content := range files {
		fullPath := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// --- Detection tests ---

func TestDetectFastAPI(t *testing.T) {
	dir := setupProject(t, map[string]string{
		"requirements.txt": "fastapi==0.100.0\nuvicorn\n",
	})
	if !detectFastAPI(dir) {
		t.Error("expected FastAPI detected")
	}
}

func TestDetectFastAPIPyproject(t *testing.T) {
	dir := setupProject(t, map[string]string{
		"pyproject.toml": `[project]
dependencies = ["fastapi>=0.100"]
`,
	})
	if !detectFastAPI(dir) {
		t.Error("expected FastAPI detected from pyproject.toml")
	}
}

func TestDetectHuma(t *testing.T) {
	dir := setupProject(t, map[string]string{
		"go.mod": `module example.com/myapp
require github.com/danielgtaylor/huma/v2 v2.10.0
`,
	})
	if !detectHuma(dir) {
		t.Error("expected Huma detected")
	}
}

func TestDetectNone(t *testing.T) {
	dir := setupProject(t, map[string]string{
		"README.md": "just a readme",
	})
	_, _, err := Infer(dir)
	if err == nil {
		t.Error("expected error when no framework detected")
	}
}

// --- Common helper tests ---

func TestPathParams(t *testing.T) {
	tests := []struct {
		path string
		want []string
	}{
		{"/users/{id}", []string{"id"}},
		{"/users/{user_id}/posts/{post_id}", []string{"user_id", "post_id"}},
		{"/users/{user-id}", []string{"user-id"}},
		{"/users", nil},
	}
	for _, tt := range tests {
		got := pathParams(tt.path)
		if len(got) == 0 && len(tt.want) == 0 {
			continue
		}
		if len(got) != len(tt.want) {
			t.Errorf("pathParams(%q) = %v, want %v", tt.path, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("pathParams(%q)[%d] = %q, want %q", tt.path, i, got[i], tt.want[i])
			}
		}
	}
}

func TestIsPrimitive(t *testing.T) {
	for _, p := range []string{"string", "integer", "number", "boolean"} {
		if !isPrimitive(p) {
			t.Errorf("isPrimitive(%q) should be true", p)
		}
	}
	for _, p := range []string{"User", "object", ""} {
		if isPrimitive(p) {
			t.Errorf("isPrimitive(%q) should be false", p)
		}
	}
}

func TestFindFiles(t *testing.T) {
	dir := setupProject(t, map[string]string{
		"main.py":                        "# app",
		"lib/utils.py":                   "# utils",
		"node_modules/pkg/index.js":      "// skip",
		"__pycache__/mod.cpython-311.py": "# skip",
		".git/config":                    "# skip",
	})
	files := findFiles(dir, ".py")
	if len(files) != 2 {
		t.Errorf("findFiles found %d files, want 2: %v", len(files), files)
	}
}

// --- InferWithFramework tests ---

func TestInferWithFrameworkUnsupported(t *testing.T) {
	_, err := InferWithFramework(t.TempDir(), "django")
	if err == nil {
		t.Error("expected error for unsupported framework")
	}
}

func TestInferWithFrameworkHuma(t *testing.T) {
	dir := setupProject(t, map[string]string{
		"go.mod": "module example.com/test\nrequire github.com/danielgtaylor/huma/v2 v2.10.0\n",
		"main.go": `package main

import "github.com/danielgtaylor/huma/v2"

func main() {
	huma.Get(api, "/items", handler)
}
`,
	})

	spec, err := InferWithFramework(dir, Huma)
	if err != nil {
		t.Fatalf("InferWithFramework error: %v", err)
	}
	if !strings.Contains(spec, "/items") {
		t.Errorf("spec missing /items")
	}
}

func TestReformatSpecJSON(t *testing.T) {
	jsonSpec := `{"openapi":"3.1.0","info":{"title":"Test","version":"1.0"},"paths":{"/health":{"get":{"responses":{"200":{"description":"ok"}}}}}}`
	out, err := reformatSpec(jsonSpec, FormatJSON)
	if err != nil {
		t.Fatalf("reformatSpec error: %v", err)
	}
	if !strings.Contains(out, "\"openapi\": \"3.1.0\"") {
		t.Errorf("expected indented JSON, got:\n%s", out)
	}
}

func TestReformatSpecInvalidJSON(t *testing.T) {
	_, err := reformatSpec("not json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestFileContainsMissing(t *testing.T) {
	dir := t.TempDir()
	if fileContains(dir, "nonexistent.txt", "foo") {
		t.Error("expected false for missing file")
	}
}
