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

func TestE2EHumaBodyUnwrapping(t *testing.T) {
	bin := buildPlugin(t)
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/service\nrequire github.com/danielgtaylor/huma/v2 v2.10.0\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "handlers.go"), []byte(`package main

import "context"

type PredictInput struct {
	Body struct {
		Data []float64 `+"`"+`json:"data" doc:"Input data for prediction" minItems:"1"`+"`"+`
	}
}

type PredictOutput struct {
	Body struct {
		Prediction []float64 `+"`"+`json:"prediction" doc:"Model prediction results"`+"`"+`
		ModelID    string    `+"`"+`json:"model_id" doc:"Identifier of the model used"`+"`"+`
	}
}

func Predict(_ context.Context, input *PredictInput) (*PredictOutput, error) {
	return nil, nil
}

type ModelsOutput struct {
	Body struct {
		Models []ModelInfo `+"`"+`json:"models" doc:"List of available models"`+"`"+`
	}
}

type ModelInfo struct {
	ID      string `+"`"+`json:"id" doc:"Model identifier"`+"`"+`
	Name    string `+"`"+`json:"name" doc:"Model name"`+"`"+`
	Version string `+"`"+`json:"version" doc:"Model version"`+"`"+`
}

func ListModels(_ context.Context, _ *struct{}) (*ModelsOutput, error) {
	return nil, nil
}

type HealthOutput struct {
	Body struct {
		Status string `+"`"+`json:"status" doc:"Service health status"`+"`"+`
	}
}

func HealthCheck(_ context.Context, _ *struct{}) (*HealthOutput, error) {
	return nil, nil
}

type Config struct {
	Host string `+"`"+`json:"host" doc:"Host address"`+"`"+`
	Port int    `+"`"+`json:"port" doc:"Port to listen on"`+"`"+`
}
`), 0o644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main

import "github.com/danielgtaylor/huma/v2"

func main() {
	cfg := huma.DefaultConfig("Runtime Service", "1.0.0")
	_ = cfg
	huma.Post(api, "/predict", Predict)
	huma.Get(api, "/models", ListModels)
	huma.Get(api, "/health", HealthCheck)
}
`), 0o644)

	resp := runPlugin(t, bin, map[string]any{
		"bundleDir": dir,
		"options":   map[string]any{"output": "api.json"},
	})

	if len(resp.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(resp.Files))
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(resp.Files[0].Content), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, resp.Files[0].Content)
	}

	// Verify schema names
	components, _ := parsed["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	if schemas == nil {
		t.Fatal("no schemas in output")
	}

	expectedSchemas := []string{
		"PredictInputBody", "PredictOutputBody",
		"ModelsOutputBody", "ModelInfo",
		"HealthOutputBody",
		"ErrorModel", "ErrorDetail",
	}
	for _, name := range expectedSchemas {
		if schemas[name] == nil {
			t.Errorf("missing schema %q", name)
		}
	}

	// Config should NOT be in schemas
	if schemas["Config"] != nil {
		t.Error("Config should not appear in schemas")
	}

	// Verify route structure
	paths, _ := parsed["paths"].(map[string]any)
	predict, _ := paths["/predict"].(map[string]any)
	post, _ := predict["post"].(map[string]any)
	if post == nil {
		t.Fatal("missing POST /predict")
	}

	// Check operationId
	if post["operationId"] != "post-predict" {
		t.Errorf("operationId = %v", post["operationId"])
	}

	// Check requestBody has $ref to PredictInputBody
	reqBody, _ := post["requestBody"].(map[string]any)
	if reqBody == nil {
		t.Fatal("missing requestBody")
	}
	content, _ := reqBody["content"].(map[string]any)
	appJSON, _ := content["application/json"].(map[string]any)
	schema, _ := appJSON["schema"].(map[string]any)
	if schema["$ref"] != "#/components/schemas/PredictInputBody" {
		t.Errorf("requestBody ref = %v", schema["$ref"])
	}

	// Check 200 response has $ref to PredictOutputBody
	responses, _ := post["responses"].(map[string]any)
	ok200, _ := responses["200"].(map[string]any)
	content200, _ := ok200["content"].(map[string]any)
	appJSON200, _ := content200["application/json"].(map[string]any)
	schema200, _ := appJSON200["schema"].(map[string]any)
	if schema200["$ref"] != "#/components/schemas/PredictOutputBody" {
		t.Errorf("response ref = %v", schema200["$ref"])
	}

	// Check default error response
	defaultResp, _ := responses["default"].(map[string]any)
	if defaultResp == nil {
		t.Fatal("missing default error response")
	}
	defaultContent, _ := defaultResp["content"].(map[string]any)
	problemJSON, _ := defaultContent["application/problem+json"].(map[string]any)
	errorSchema, _ := problemJSON["schema"].(map[string]any)
	if errorSchema["$ref"] != "#/components/schemas/ErrorModel" {
		t.Errorf("error ref = %v", errorSchema["$ref"])
	}

	// Verify PredictInputBody has proper field details
	pib, _ := schemas["PredictInputBody"].(map[string]any)
	pibProps, _ := pib["properties"].(map[string]any)
	dataField, _ := pibProps["data"].(map[string]any)
	if dataField["description"] != "Input data for prediction" {
		t.Errorf("data description = %v", dataField["description"])
	}
	if dataField["minItems"] != float64(1) {
		t.Errorf("data minItems = %v", dataField["minItems"])
	}

	// Check additionalProperties on body schemas
	if pib["additionalProperties"] != false {
		t.Error("PredictInputBody missing additionalProperties: false")
	}

	// GET /health should have no requestBody
	health, _ := paths["/health"].(map[string]any)
	getHealth, _ := health["get"].(map[string]any)
	if getHealth["requestBody"] != nil {
		t.Error("GET /health should not have requestBody")
	}
	// But should have response body
	healthResp, _ := getHealth["responses"].(map[string]any)
	health200, _ := healthResp["200"].(map[string]any)
	if health200["content"] == nil {
		t.Error("GET /health should have response content")
	}
}
