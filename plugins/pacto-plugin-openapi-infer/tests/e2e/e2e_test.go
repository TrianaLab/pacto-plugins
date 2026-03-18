//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type generatedFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type generateResponse struct {
	Files   []generatedFile `json:"files"`
	Message string          `json:"message,omitempty"`
}

func buildPlugin(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "pacto-plugin-openapi-infer")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/")
	cmd.Dir = filepath.Join("..", "..")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}

func runPlugin(t *testing.T, bin string, input map[string]any) generateResponse {
	t.Helper()
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}

	cmd := exec.Command(bin)
	cmd.Stdin = bytes.NewReader(inputJSON)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("plugin failed: %v\nstderr: %s", err, stderr.String())
	}

	var resp generateResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal output: %v\nraw: %s", err, stdout.String())
	}
	return resp
}

func TestE2EHumaProject(t *testing.T) {
	bin := buildPlugin(t)
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/petstore\nrequire github.com/danielgtaylor/huma/v2 v2.10.0\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main

import "github.com/danielgtaylor/huma/v2"

type Pet struct {
	ID   int64  `+"`"+`json:"id"`+"`"+`
	Name string `+"`"+`json:"name"`+"`"+`
}

func main() {
	cfg := huma.DefaultConfig("Pet Store", "1.0.0")
	_ = cfg
	huma.Get(api, "/pets", listPets)
	huma.Post(api, "/pets", createPet)
	huma.Get(api, "/pets/{petId}", getPet)
}
`), 0o644)

	resp := runPlugin(t, bin, map[string]any{
		"bundleDir": dir,
	})

	if len(resp.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(resp.Files))
	}
	if resp.Files[0].Path != "interfaces/openapi.yaml" {
		t.Errorf("path = %q", resp.Files[0].Path)
	}

	content := resp.Files[0].Content
	for _, want := range []string{"openapi:", "Pet Store", "/pets", "get:", "post:"} {
		if !strings.Contains(content, want) {
			t.Errorf("spec missing %q", want)
		}
	}

	if !strings.Contains(resp.Message, "huma") {
		t.Errorf("message should mention huma, got: %s", resp.Message)
	}
}

func TestE2EHumaJSONOutput(t *testing.T) {
	bin := buildPlugin(t)
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\nrequire github.com/danielgtaylor/huma/v2 v2.10.0\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main

import "github.com/danielgtaylor/huma/v2"

func main() {
	huma.Get(api, "/health", handler)
}
`), 0o644)

	resp := runPlugin(t, bin, map[string]any{
		"bundleDir": dir,
		"options":   map[string]any{"output": "api.json"},
	})

	if len(resp.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(resp.Files))
	}
	if resp.Files[0].Path != "api.json" {
		t.Errorf("path = %q", resp.Files[0].Path)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(resp.Files[0].Content), &parsed); err != nil {
		t.Fatalf("output should be valid JSON: %v", err)
	}
	if parsed["openapi"] != "3.1.0" {
		t.Errorf("openapi = %v", parsed["openapi"])
	}
}

func TestE2ENoFrameworkDetected(t *testing.T) {
	bin := buildPlugin(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("nothing here"), 0o644)

	inputJSON, _ := json.Marshal(map[string]any{"bundleDir": dir})
	cmd := exec.Command(bin)
	cmd.Stdin = bytes.NewReader(inputJSON)
	err := cmd.Run()
	if err == nil {
		t.Error("expected error when no framework detected")
	}
}

func TestE2EFrameworkOverride(t *testing.T) {
	bin := buildPlugin(t)
	dir := t.TempDir()

	// No go.mod with huma, but we override framework
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main

import "github.com/danielgtaylor/huma/v2"

func main() {
	huma.Get(api, "/items", handler)
}
`), 0o644)

	resp := runPlugin(t, bin, map[string]any{
		"bundleDir": dir,
		"options":   map[string]any{"framework": "huma"},
	})

	if len(resp.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(resp.Files))
	}
	if !strings.Contains(resp.Files[0].Content, "/items") {
		t.Error("spec missing /items")
	}
}
