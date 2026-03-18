//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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
	bin := filepath.Join(t.TempDir(), "pacto-plugin-schema-infer")
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

func TestE2EJSON(t *testing.T) {
	bin := buildPlugin(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"host":"localhost","port":8080,"debug":true}`), 0o644)

	resp := runPlugin(t, bin, map[string]any{
		"bundleDir": dir,
		"options":   map[string]any{"file": "config.json"},
	})

	if len(resp.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(resp.Files))
	}
	if resp.Files[0].Path != "config.schema.json" {
		t.Errorf("path = %q", resp.Files[0].Path)
	}

	var schema map[string]any
	if err := json.Unmarshal([]byte(resp.Files[0].Content), &schema); err != nil {
		t.Fatalf("invalid JSON schema: %v", err)
	}
	if schema["type"] != "object" {
		t.Errorf("type = %v", schema["type"])
	}
	props := schema["properties"].(map[string]any)
	if props["host"].(map[string]any)["type"] != "string" {
		t.Error("host should be string")
	}
	if props["port"].(map[string]any)["type"] != "number" {
		t.Error("port should be number")
	}
	if props["debug"].(map[string]any)["type"] != "boolean" {
		t.Error("debug should be boolean")
	}
}

func TestE2EYAML(t *testing.T) {
	bin := buildPlugin(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("name: myservice\nreplicas: 3\n"), 0o644)

	resp := runPlugin(t, bin, map[string]any{
		"bundleDir": dir,
		"options":   map[string]any{"file": "config.yaml"},
	})

	if len(resp.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(resp.Files))
	}

	var schema map[string]any
	if err := json.Unmarshal([]byte(resp.Files[0].Content), &schema); err != nil {
		t.Fatalf("invalid JSON schema: %v", err)
	}
	props := schema["properties"].(map[string]any)
	if props["name"].(map[string]any)["type"] != "string" {
		t.Error("name should be string")
	}
}

func TestE2ETOML(t *testing.T) {
	bin := buildPlugin(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.toml"), []byte("[server]\nhost = \"0.0.0.0\"\nport = 9090\n"), 0o644)

	resp := runPlugin(t, bin, map[string]any{
		"bundleDir": dir,
		"options":   map[string]any{"file": "config.toml"},
	})

	if len(resp.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(resp.Files))
	}

	var schema map[string]any
	if err := json.Unmarshal([]byte(resp.Files[0].Content), &schema); err != nil {
		t.Fatalf("invalid JSON schema: %v", err)
	}
	if schema["type"] != "object" {
		t.Errorf("type = %v", schema["type"])
	}
}

func TestE2EMissingFileOption(t *testing.T) {
	bin := buildPlugin(t)
	inputJSON, _ := json.Marshal(map[string]any{
		"bundleDir": t.TempDir(),
		"options":   map[string]any{},
	})

	cmd := exec.Command(bin)
	cmd.Stdin = bytes.NewReader(inputJSON)
	err := cmd.Run()
	if err == nil {
		t.Error("expected error when file option is missing")
	}
}
