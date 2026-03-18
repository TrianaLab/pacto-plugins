package infer

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleHuma = `package main

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type Pet struct {
	ID   int64  ` + "`" + `json:"id"` + "`" + `
	Name string ` + "`" + `json:"name"` + "`" + `
	Tag  string ` + "`" + `json:"tag"` + "`" + `
}

type PetCreate struct {
	Name string ` + "`" + `json:"name"` + "`" + `
	Tag  string ` + "`" + `json:"tag"` + "`" + `
}

func RegisterRoutes(api huma.API) {
	huma.Get(api, "/pets", listPets)
	huma.Get(api, "/pets/{pet-id}", getPet)
	huma.Post(api, "/pets", createPet)
	huma.Delete(api, "/pets/{pet-id}", deletePet)

	huma.Register(api, huma.Operation{
		Method:  http.MethodGet,
		Path:    "/pets/{pet-id}/owner",
		Summary: "Get pet owner",
	}, getPetOwner)
}
`

// sampleHumaService is a realistic Huma service with Body struct conventions.
const sampleHumaService = `package service

import "context"

type PredictInput struct {
	Body struct {
		Data []float64 ` + "`" + `json:"data" doc:"Input data for prediction" minItems:"1"` + "`" + `
	}
}

type PredictOutput struct {
	Body struct {
		Prediction []float64 ` + "`" + `json:"prediction" doc:"Model prediction results"` + "`" + `
		ModelID    string    ` + "`" + `json:"model_id" doc:"Identifier of the model used"` + "`" + `
	}
}

func Predict(_ context.Context, input *PredictInput) (*PredictOutput, error) {
	return nil, nil
}

type ModelsOutput struct {
	Body struct {
		Models []ModelInfo ` + "`" + `json:"models" doc:"List of available models"` + "`" + `
	}
}

type ModelInfo struct {
	ID      string ` + "`" + `json:"id" doc:"Model identifier"` + "`" + `
	Name    string ` + "`" + `json:"name" doc:"Model name"` + "`" + `
	Version string ` + "`" + `json:"version" doc:"Model version"` + "`" + `
}

func ListModels(_ context.Context, _ *struct{}) (*ModelsOutput, error) {
	return nil, nil
}

type HealthOutput struct {
	Body struct {
		Status string ` + "`" + `json:"status" doc:"Service health status"` + "`" + `
	}
}

func HealthCheck(_ context.Context, _ *struct{}) (*HealthOutput, error) {
	return nil, nil
}

type Config struct {
	Host string ` + "`" + `json:"host" doc:"Host address"` + "`" + `
	Port int    ` + "`" + `json:"port" doc:"Port to listen on"` + "`" + `
}
`

const sampleHumaRoutes = `package main

import "github.com/danielgtaylor/huma/v2"

func RegisterRoutes(api huma.API) {
	huma.Post(api, "/predict", runtimeInternal.Predict)
	huma.Get(api, "/models", runtimeInternal.ListModels)
	huma.Get(api, "/health", runtimeInternal.HealthCheck)
}
`

func parseSource(t *testing.T, src string) *parseResult {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return &parseResult{file: file}
}

type parseResult struct {
	file *ast.File
}

func TestParseHumaRoutesAST(t *testing.T) {
	pr := parseSource(t, sampleHuma)
	routes := parseHumaRoutesAST(pr.file)
	if len(routes) != 5 {
		t.Fatalf("got %d routes, want 5", len(routes))
	}

	// huma.Get /pets
	r := routes[0]
	if r.Method != "GET" || r.Path != "/pets" {
		t.Errorf("route 0: %s %s", r.Method, r.Path)
	}

	// huma.Get /pets/{pet-id}
	r = routes[1]
	if r.Method != "GET" || r.Path != "/pets/{pet-id}" {
		t.Errorf("route 1: %s %s", r.Method, r.Path)
	}
	if len(r.Params) != 1 || r.Params[0].Name != "pet-id" {
		t.Errorf("params = %+v", r.Params)
	}

	// huma.Post /pets
	r = routes[2]
	if r.Method != "POST" || r.Path != "/pets" {
		t.Errorf("route 2: %s %s", r.Method, r.Path)
	}

	// huma.Delete /pets/{pet-id}
	r = routes[3]
	if r.Method != "DELETE" || r.Path != "/pets/{pet-id}" {
		t.Errorf("route 3: %s %s", r.Method, r.Path)
	}

	// huma.Register /pets/{pet-id}/owner
	r = routes[4]
	if r.Method != "GET" || r.Path != "/pets/{pet-id}/owner" {
		t.Errorf("route 4: %s %s", r.Method, r.Path)
	}
	if r.Summary != "Get pet owner" {
		t.Errorf("summary = %q", r.Summary)
	}
}

func TestParseGoStructsAST(t *testing.T) {
	pr := parseSource(t, sampleHuma)
	schemas := parseGoStructsAST(pr.file)
	if len(schemas) != 2 {
		t.Fatalf("got %d schemas, want 2", len(schemas))
	}

	pet := schemas[0]
	if pet.Name != "Pet" {
		t.Errorf("name = %q", pet.Name)
	}
	if len(pet.Fields) != 3 {
		t.Fatalf("Pet has %d fields, want 3", len(pet.Fields))
	}
	if pet.Fields[0].Name != "id" || pet.Fields[0].Type != "integer" {
		t.Errorf("field 0 = %+v", pet.Fields[0])
	}
	if pet.Fields[1].Name != "name" || pet.Fields[1].Type != "string" {
		t.Errorf("field 1 = %+v", pet.Fields[1])
	}
}

func TestParseGoStructsASTSkipsUnexported(t *testing.T) {
	src := `package main

type internal struct {
	field string
}

type Public struct {
	Name string ` + "`" + `json:"name"` + "`" + `
}
`
	pr := parseSource(t, src)
	schemas := parseGoStructsAST(pr.file)
	if len(schemas) != 1 {
		t.Fatalf("got %d schemas, want 1", len(schemas))
	}
	if schemas[0].Name != "Public" {
		t.Errorf("name = %q", schemas[0].Name)
	}
}

func TestParseGoStructsASTArrayField(t *testing.T) {
	src := `package main

type Owner struct {
	Name string ` + "`" + `json:"name"` + "`" + `
	Pets []Pet  ` + "`" + `json:"pets"` + "`" + `
}
`
	pr := parseSource(t, src)
	schemas := parseGoStructsAST(pr.file)
	if len(schemas) != 1 || len(schemas[0].Fields) != 2 {
		t.Fatalf("unexpected schemas: %+v", schemas)
	}
	if !schemas[0].Fields[1].IsArray {
		t.Error("pets should be an array")
	}
}

func TestParseGoStructsASTJsonDash(t *testing.T) {
	src := `package main

type Thing struct {
	Name   string ` + "`" + `json:"name"` + "`" + `
	Secret string ` + "`" + `json:"-"` + "`" + `
}
`
	pr := parseSource(t, src)
	schemas := parseGoStructsAST(pr.file)
	if len(schemas) != 1 || len(schemas[0].Fields) != 1 {
		t.Fatalf("expected 1 schema with 1 field, got %+v", schemas)
	}
	if schemas[0].Fields[0].Name != "name" {
		t.Errorf("field name = %q", schemas[0].Fields[0].Name)
	}
}

func TestParseHumaInfoAST(t *testing.T) {
	src := `package main
func main() {
	cfg := huma.DefaultConfig("Pet Store", "1.0.0")
	_ = cfg
}
`
	pr := parseSource(t, src)
	info, ok := parseHumaInfoAST(pr.file)
	if !ok {
		t.Fatal("expected info parsed")
	}
	if info.Title != "Pet Store" || info.Version != "1.0.0" {
		t.Errorf("info = %+v", info)
	}
}

func TestResolveGoFieldTypeSelectorExpr(t *testing.T) {
	src := `package main

type Thing struct {
	Created time.Time ` + "`" + `json:"created"` + "`" + `
}
`
	pr := parseSource(t, src)
	schemas := parseGoStructsAST(pr.file)
	if len(schemas) != 1 || len(schemas[0].Fields) != 1 {
		t.Fatalf("unexpected schemas: %+v", schemas)
	}
	// time.Time should become "time.Time" (non-primitive → schema ref)
	if schemas[0].Fields[0].Type != "time.Time" {
		t.Errorf("type = %q, want time.Time", schemas[0].Fields[0].Type)
	}
}

func TestResolveGoFieldTypePointer(t *testing.T) {
	src := `package main

type Thing struct {
	Name *string ` + "`" + `json:"name"` + "`" + `
}
`
	pr := parseSource(t, src)
	schemas := parseGoStructsAST(pr.file)
	if len(schemas) != 1 || len(schemas[0].Fields) != 1 {
		t.Fatalf("unexpected schemas: %+v", schemas)
	}
	if schemas[0].Fields[0].Type != "string" {
		t.Errorf("type = %q, want string", schemas[0].Fields[0].Type)
	}
}

func TestResolveHTTPMethodStringLiteral(t *testing.T) {
	src := `package main

import "github.com/danielgtaylor/huma/v2"

func register(api huma.API) {
	huma.Register(api, huma.Operation{
		Method: "post",
		Path:   "/items",
	}, handler)
}
`
	pr := parseSource(t, src)
	routes := parseHumaRoutesAST(pr.file)
	if len(routes) != 1 {
		t.Fatalf("got %d routes, want 1", len(routes))
	}
	if routes[0].Method != "POST" {
		t.Errorf("method = %q, want POST", routes[0].Method)
	}
}

func TestParseHumaOperationMissingFields(t *testing.T) {
	// Operation with only Method, no Path → should return false
	src := `package main

import "github.com/danielgtaylor/huma/v2"

func register(api huma.API) {
	huma.Register(api, huma.Operation{
		Method: "GET",
	}, handler)
}
`
	pr := parseSource(t, src)
	routes := parseHumaRoutesAST(pr.file)
	if len(routes) != 0 {
		t.Errorf("got %d routes, want 0 (missing path)", len(routes))
	}
}

func TestParseHumaRoutesNonHumaSelector(t *testing.T) {
	src := `package main

func register() {
	other.Get(api, "/foo", handler)
}
`
	pr := parseSource(t, src)
	routes := parseHumaRoutesAST(pr.file)
	if len(routes) != 0 {
		t.Errorf("got %d routes, want 0", len(routes))
	}
}

func TestParseGoStructsEmbeddedField(t *testing.T) {
	src := `package main

type Base struct {
	ID int ` + "`" + `json:"id"` + "`" + `
}

type Thing struct {
	Base
	Name string ` + "`" + `json:"name"` + "`" + `
}
`
	pr := parseSource(t, src)
	schemas := parseGoStructsAST(pr.file)
	// Base has 1 field (ID), Thing has 1 field (Name) — embedded Base is skipped
	if len(schemas) != 2 {
		t.Fatalf("got %d schemas, want 2", len(schemas))
	}
	for _, s := range schemas {
		if s.Name == "Thing" && len(s.Fields) != 1 {
			t.Errorf("Thing should have 1 field, got %d", len(s.Fields))
		}
	}
}

func TestParseGoStructsNoTag(t *testing.T) {
	src := `package main

type Item struct {
	Name string
}
`
	pr := parseSource(t, src)
	schemas := parseGoStructsAST(pr.file)
	if len(schemas) != 1 || len(schemas[0].Fields) != 1 {
		t.Fatalf("unexpected: %+v", schemas)
	}
	// No json tag → field name used as-is
	if schemas[0].Fields[0].Name != "Name" {
		t.Errorf("field name = %q, want Name", schemas[0].Fields[0].Name)
	}
}

func TestParseHumaInfoASTNotFound(t *testing.T) {
	src := `package main

func main() {
	x := 1
	_ = x
}
`
	pr := parseSource(t, src)
	_, ok := parseHumaInfoAST(pr.file)
	if ok {
		t.Error("expected no info found")
	}
}

func TestStringLitValueNonString(t *testing.T) {
	src := `package main

var x = 42
`
	pr := parseSource(t, src)
	// Find the literal 42 in the AST
	for _, decl := range pr.file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Values) == 0 {
				continue
			}
			result := stringLitValue(vs.Values[0])
			if result != "" {
				t.Errorf("expected empty, got %q", result)
			}
		}
	}
}

func TestMapGoType(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"string", "string"},
		{"int", "integer"},
		{"int64", "integer"},
		{"float64", "number"},
		{"bool", "boolean"},
		{"Pet", "Pet"},
	}
	for _, tt := range tests {
		got := mapGoType(tt.input)
		if got != tt.want {
			t.Errorf("mapGoType(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseHumaInfoNonIdentSelector(t *testing.T) {
	// sel.X is not an Ident (it's a function call result)
	src := `package main

func main() {
	cfg := getPackage().DefaultConfig("Test", "1.0")
	_ = cfg
}
`
	pr := parseSource(t, src)
	_, ok := parseHumaInfoAST(pr.file)
	if ok {
		t.Error("expected no info for non-ident selector")
	}
}

func TestParseHumaRoutesNonIdentSelectorX(t *testing.T) {
	// sel.X is a call expression, not an Ident
	src := `package main

func register() {
	getAPI().Get(api, "/foo", handler)
}
`
	pr := parseSource(t, src)
	routes := parseHumaRoutesAST(pr.file)
	if len(routes) != 0 {
		t.Errorf("got %d routes, want 0", len(routes))
	}
}

func TestExtractHumaSkipsBadFiles(t *testing.T) {
	dir := setupProject(t, map[string]string{
		"go.mod":  "module test\nrequire github.com/danielgtaylor/huma/v2 v2.10.0\n",
		"bad.go":  "this is not valid go code {{{{",
		"good.go": "package main\nimport \"github.com/danielgtaylor/huma/v2\"\nfunc init() { huma.Get(api, \"/ok\", h) }\n",
	})
	result, err := extractHuma(dir)
	if err != nil {
		t.Fatalf("extractHuma error: %v", err)
	}
	if len(result.Routes) != 1 {
		t.Errorf("got %d routes, want 1", len(result.Routes))
	}
}

func TestExtractHumaUnreadableFile(t *testing.T) {
	dir := setupProject(t, map[string]string{
		"go.mod":    "module test\nrequire github.com/danielgtaylor/huma/v2 v2.10.0\n",
		"unread.go": "package main\n",
		"good.go":   "package main\nimport \"github.com/danielgtaylor/huma/v2\"\nfunc init() { huma.Get(api, \"/ok\", h) }\n",
	})
	// Make unread.go unreadable
	_ = os.Chmod(filepath.Join(dir, "unread.go"), 0o000)
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(dir, "unread.go"), 0o644) })

	result, err := extractHuma(dir)
	if err != nil {
		t.Fatalf("extractHuma error: %v", err)
	}
	if len(result.Routes) != 1 {
		t.Errorf("got %d routes, want 1", len(result.Routes))
	}
}

func TestParseHumaOperationNonCompositeLit(t *testing.T) {
	// huma.Register(api, someVariable, handler) — not a composite literal
	src := `package main

import "github.com/danielgtaylor/huma/v2"

func register(api huma.API) {
	huma.Register(api, existingOp, handler)
}
`
	pr := parseSource(t, src)
	routes := parseHumaRoutesAST(pr.file)
	if len(routes) != 0 {
		t.Errorf("got %d routes, want 0", len(routes))
	}
}

func TestResolveHTTPMethodUnknown(t *testing.T) {
	src := `package main

import "github.com/danielgtaylor/huma/v2"

func register(api huma.API) {
	huma.Register(api, huma.Operation{
		Method: someVar,
		Path:   "/test",
	}, handler)
}
`
	pr := parseSource(t, src)
	routes := parseHumaRoutesAST(pr.file)
	// Method is a variable reference, not resolvable → empty string → route rejected
	if len(routes) != 0 {
		t.Errorf("got %d routes, want 0", len(routes))
	}
}

func TestParseGoStructsNonStructType(t *testing.T) {
	src := `package main

type MyString string
type MyInt int
type Actual struct {
	Name string ` + "`" + `json:"name"` + "`" + `
}
`
	pr := parseSource(t, src)
	schemas := parseGoStructsAST(pr.file)
	if len(schemas) != 1 || schemas[0].Name != "Actual" {
		t.Fatalf("expected only Actual struct, got %+v", schemas)
	}
}

func TestParseGoStructsUnexportedFieldName(t *testing.T) {
	src := `package main

type Thing struct {
	Name     string ` + "`" + `json:"name"` + "`" + `
	internal string
}
`
	pr := parseSource(t, src)
	schemas := parseGoStructsAST(pr.file)
	if len(schemas) != 1 || len(schemas[0].Fields) != 1 {
		t.Fatalf("expected 1 schema with 1 field, got %+v", schemas)
	}
}

func TestResolveGoFieldTypeMapInterface(t *testing.T) {
	src := `package main

type Thing struct {
	Data map[string]any ` + "`" + `json:"data"` + "`" + `
}
`
	pr := parseSource(t, src)
	schemas := parseGoStructsAST(pr.file)
	if len(schemas) != 1 || len(schemas[0].Fields) != 1 {
		t.Fatalf("unexpected: %+v", schemas)
	}
	// map type falls through to default → "string"
	if schemas[0].Fields[0].Type != "string" {
		t.Errorf("type = %q", schemas[0].Fields[0].Type)
	}
}

func TestParseHumaInfoNotDefaultConfig(t *testing.T) {
	src := `package main

func main() {
	cfg := huma.NewAPI("test", "1.0")
	_ = cfg
}
`
	pr := parseSource(t, src)
	_, ok := parseHumaInfoAST(pr.file)
	if ok {
		t.Error("expected no info — not DefaultConfig")
	}
}

func TestStringLitValueUnquoteError(t *testing.T) {
	// Construct a BasicLit with a malformed string value
	lit := &ast.BasicLit{Kind: token.STRING, Value: `"unclosed`}
	result := stringLitValue(lit)
	if result != "unclosed" {
		t.Errorf("got %q, want %q", result, "unclosed")
	}
}

func TestParseHumaOperationNonKVElements(t *testing.T) {
	// Construct an Operation composite lit with positional elements (not key-value)
	comp := &ast.CompositeLit{
		Elts: []ast.Expr{
			&ast.BasicLit{Kind: token.STRING, Value: `"GET"`},
		},
	}
	_, ok := parseHumaOperation(comp)
	if ok {
		t.Error("expected false for positional elements")
	}
}

func TestParseHumaOperationNonIdentKey(t *testing.T) {
	// Construct a key-value with non-Ident key
	comp := &ast.CompositeLit{
		Elts: []ast.Expr{
			&ast.KeyValueExpr{
				Key:   &ast.BasicLit{Kind: token.STRING, Value: `"Method"`},
				Value: &ast.BasicLit{Kind: token.STRING, Value: `"GET"`},
			},
		},
	}
	_, ok := parseHumaOperation(comp)
	if ok {
		t.Error("expected false for non-ident key")
	}
}

func TestExtractHumaIntegration(t *testing.T) {
	dir := setupProject(t, map[string]string{
		"go.mod": "module example.com/petstore\nrequire github.com/danielgtaylor/huma/v2 v2.10.0\n",
		"main.go": `package main

import "github.com/danielgtaylor/huma/v2"

type Pet struct {
	ID   int64  ` + "`" + `json:"id"` + "`" + `
	Name string ` + "`" + `json:"name"` + "`" + `
	Tag  string ` + "`" + `json:"tag"` + "`" + `
}

func main() {
	cfg := huma.DefaultConfig("Pet Store", "1.0.0")
	_ = cfg
	huma.Get(api, "/pets", listPets)
	huma.Get(api, "/pets/{pet-id}", getPet)
	huma.Post(api, "/pets", createPet)
	huma.Delete(api, "/pets/{pet-id}", deletePet)
}
`,
	})

	spec, fw, err := Infer(dir)
	if err != nil {
		t.Fatalf("Infer error: %v", err)
	}
	if fw != Huma {
		t.Errorf("framework = %q", fw)
	}

	for _, expected := range []string{
		"title: Pet Store",
		"/pets:",
		"/pets/{pet-id}:",
		"get:",
		"post:",
		"delete:",
	} {
		if !strings.Contains(spec, expected) {
			t.Errorf("spec missing %q\n%s", expected, spec)
		}
	}
}

// --- New tests for Huma Body unwrapping and handler linking ---

func TestExtractHumaHandlerMap(t *testing.T) {
	pr := parseSource(t, sampleHumaRoutes)
	handlers := extractHumaHandlerMap(pr.file)

	tests := []struct {
		method, path, handler string
	}{
		{"POST", "/predict", "Predict"},
		{"GET", "/models", "ListModels"},
		{"GET", "/health", "HealthCheck"},
	}
	for _, tt := range tests {
		key := routeKey{tt.method, tt.path}
		got, ok := handlers[key]
		if !ok {
			t.Errorf("missing handler for %s %s", tt.method, tt.path)
			continue
		}
		if got != tt.handler {
			t.Errorf("%s %s: handler = %q, want %q", tt.method, tt.path, got, tt.handler)
		}
	}
}

func TestExtractHumaHandlerMapRegister(t *testing.T) {
	src := `package main

import (
	"net/http"
	"github.com/danielgtaylor/huma/v2"
)

func register(api huma.API) {
	huma.Register(api, huma.Operation{
		Method: http.MethodGet,
		Path:   "/items",
	}, listItems)
}
`
	pr := parseSource(t, src)
	handlers := extractHumaHandlerMap(pr.file)
	key := routeKey{"GET", "/items"}
	if name, ok := handlers[key]; !ok || name != "listItems" {
		t.Errorf("handler = %q, want listItems", name)
	}
}

func TestParseFuncSignatures(t *testing.T) {
	pr := parseSource(t, sampleHumaService)
	sigs := parseFuncSignatures(pr.file)

	sigMap := map[string]funcSig{}
	for _, s := range sigs {
		sigMap[s.name] = s
	}

	// Predict: input=PredictInput, output=PredictOutput
	if sig, ok := sigMap["Predict"]; !ok {
		t.Error("missing Predict signature")
	} else {
		if sig.inputType != "PredictInput" {
			t.Errorf("Predict input = %q", sig.inputType)
		}
		if sig.outputType != "PredictOutput" {
			t.Errorf("Predict output = %q", sig.outputType)
		}
	}

	// ListModels: input="" (struct{}), output=ModelsOutput
	if sig, ok := sigMap["ListModels"]; !ok {
		t.Error("missing ListModels signature")
	} else {
		if sig.inputType != "" {
			t.Errorf("ListModels input = %q, want empty", sig.inputType)
		}
		if sig.outputType != "ModelsOutput" {
			t.Errorf("ListModels output = %q", sig.outputType)
		}
	}

	// HealthCheck: input="" (struct{}), output=HealthOutput
	if sig, ok := sigMap["HealthCheck"]; !ok {
		t.Error("missing HealthCheck signature")
	} else {
		if sig.inputType != "" {
			t.Errorf("HealthCheck input = %q, want empty", sig.inputType)
		}
		if sig.outputType != "HealthOutput" {
			t.Errorf("HealthCheck output = %q", sig.outputType)
		}
	}
}

func TestCollectStructTypes(t *testing.T) {
	pr := parseSource(t, sampleHumaService)
	structMap := map[string]*ast.StructType{}
	collectStructTypes(pr.file, structMap)

	expected := []string{"PredictInput", "PredictOutput", "ModelsOutput", "ModelInfo", "HealthOutput", "Config"}
	for _, name := range expected {
		if _, ok := structMap[name]; !ok {
			t.Errorf("missing struct %q", name)
		}
	}
}

func TestProcessBodyType(t *testing.T) {
	pr := parseSource(t, sampleHumaService)
	structMap := map[string]*ast.StructType{}
	collectStructTypes(pr.file, structMap)

	result := &Result{}
	added := map[string]bool{}

	// Process PredictInput → should create PredictInputBody
	bodyName := processBodyType("PredictInput", structMap, result, added)
	if bodyName != "PredictInputBody" {
		t.Fatalf("body name = %q, want PredictInputBody", bodyName)
	}
	if len(result.Schemas) != 1 {
		t.Fatalf("expected 1 schema, got %d", len(result.Schemas))
	}

	schema := result.Schemas[0]
	if schema.Name != "PredictInputBody" {
		t.Errorf("schema name = %q", schema.Name)
	}
	if schema.AdditionalProperties == nil || *schema.AdditionalProperties != false {
		t.Error("expected additionalProperties = false")
	}

	// Should have 1 field: data
	if len(schema.Fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(schema.Fields))
	}
	f := schema.Fields[0]
	if f.Name != "data" {
		t.Errorf("field name = %q", f.Name)
	}
	if f.Type != "number" {
		t.Errorf("field type = %q, want number", f.Type)
	}
	if !f.IsArray {
		t.Error("data should be an array")
	}
	if f.Description != "Input data for prediction" {
		t.Errorf("description = %q", f.Description)
	}
	if f.Format != "double" {
		t.Errorf("format = %q, want double", f.Format)
	}
	if f.MinItems != 1 {
		t.Errorf("minItems = %d, want 1", f.MinItems)
	}
	if !f.NullableArray {
		t.Error("expected nullable array")
	}
}

func TestProcessBodyTypeDedup(t *testing.T) {
	pr := parseSource(t, sampleHumaService)
	structMap := map[string]*ast.StructType{}
	collectStructTypes(pr.file, structMap)

	result := &Result{}
	added := map[string]bool{}

	// Process twice — should not duplicate
	processBodyType("PredictInput", structMap, result, added)
	processBodyType("PredictInput", structMap, result, added)
	if len(result.Schemas) != 1 {
		t.Errorf("expected 1 schema (dedup), got %d", len(result.Schemas))
	}
}

func TestProcessBodyTypeNoBody(t *testing.T) {
	pr := parseSource(t, sampleHumaService)
	structMap := map[string]*ast.StructType{}
	collectStructTypes(pr.file, structMap)

	result := &Result{}
	added := map[string]bool{}

	// ModelInfo has no Body field
	bodyName := processBodyType("ModelInfo", structMap, result, added)
	if bodyName != "" {
		t.Errorf("expected empty, got %q", bodyName)
	}
}

func TestProcessBodyTypeUnknown(t *testing.T) {
	result := &Result{}
	added := map[string]bool{}
	bodyName := processBodyType("NonExistent", map[string]*ast.StructType{}, result, added)
	if bodyName != "" {
		t.Errorf("expected empty, got %q", bodyName)
	}
}

func TestParseHumaStructFieldDocTag(t *testing.T) {
	src := `package main

type Foo struct {
	Name string ` + "`" + `json:"name" doc:"The name"` + "`" + `
}
`
	pr := parseSource(t, src)
	structMap := map[string]*ast.StructType{}
	collectStructTypes(pr.file, structMap)

	st := structMap["Foo"]
	f, ok := parseHumaStructField(st.Fields.List[0])
	if !ok {
		t.Fatal("expected field parsed")
	}
	if f.Description != "The name" {
		t.Errorf("description = %q", f.Description)
	}
}

func TestParseHumaStructFieldFormat(t *testing.T) {
	src := `package main

type Foo struct {
	Value float64 ` + "`" + `json:"value"` + "`" + `
	Count int64   ` + "`" + `json:"count"` + "`" + `
}
`
	pr := parseSource(t, src)
	structMap := map[string]*ast.StructType{}
	collectStructTypes(pr.file, structMap)

	st := structMap["Foo"]
	f0, _ := parseHumaStructField(st.Fields.List[0])
	if f0.Format != "double" {
		t.Errorf("float64 format = %q, want double", f0.Format)
	}
	f1, _ := parseHumaStructField(st.Fields.List[1])
	if f1.Format != "int64" {
		t.Errorf("int64 format = %q, want int64", f1.Format)
	}
}

func TestParseHumaStructFieldMinItems(t *testing.T) {
	src := `package main

type Foo struct {
	Items []string ` + "`" + `json:"items" minItems:"3"` + "`" + `
}
`
	pr := parseSource(t, src)
	structMap := map[string]*ast.StructType{}
	collectStructTypes(pr.file, structMap)

	st := structMap["Foo"]
	f, ok := parseHumaStructField(st.Fields.List[0])
	if !ok {
		t.Fatal("expected field parsed")
	}
	if f.MinItems != 3 {
		t.Errorf("minItems = %d, want 3", f.MinItems)
	}
}

func TestParseHumaStructFieldJsonDash(t *testing.T) {
	src := `package main

type Foo struct {
	Secret string ` + "`" + `json:"-"` + "`" + `
}
`
	pr := parseSource(t, src)
	structMap := map[string]*ast.StructType{}
	collectStructTypes(pr.file, structMap)

	st := structMap["Foo"]
	_, ok := parseHumaStructField(st.Fields.List[0])
	if ok {
		t.Error("json:\"-\" field should be skipped")
	}
}

func TestParseHumaStructFieldUnexported(t *testing.T) {
	src := `package main

type Foo struct {
	internal string
}
`
	pr := parseSource(t, src)
	structMap := map[string]*ast.StructType{}
	collectStructTypes(pr.file, structMap)

	st := structMap["Foo"]
	_, ok := parseHumaStructField(st.Fields.List[0])
	if ok {
		t.Error("unexported field should be skipped")
	}
}

func TestGenerateOperationId(t *testing.T) {
	tests := []struct {
		method, path, want string
	}{
		{"POST", "/predict", "post-predict"},
		{"GET", "/models", "get-models"},
		{"GET", "/health", "get-health"},
		{"GET", "/pets/{pet-id}", "get-pets-pet-id"},
		{"DELETE", "/users/{id}/posts/{post-id}", "delete-users-id-posts-post-id"},
	}
	for _, tt := range tests {
		got := generateOperationId(tt.method, tt.path)
		if got != tt.want {
			t.Errorf("generateOperationId(%q, %q) = %q, want %q", tt.method, tt.path, got, tt.want)
		}
	}
}

func TestGenerateSummary(t *testing.T) {
	tests := []struct {
		method, path, want string
	}{
		{"POST", "/predict", "Post predict"},
		{"GET", "/models", "Get models"},
		{"GET", "/health", "Get health"},
		{"GET", "/pets/{pet-id}", "Get pets pet-id"},
	}
	for _, tt := range tests {
		got := generateSummary(tt.method, tt.path)
		if got != tt.want {
			t.Errorf("generateSummary(%q, %q) = %q, want %q", tt.method, tt.path, got, tt.want)
		}
	}
}

func TestFuncNameFromExpr(t *testing.T) {
	src := `package main

func register() {
	huma.Get(api, "/a", handler)
	huma.Post(api, "/b", pkg.Handler)
}
`
	pr := parseSource(t, src)
	handlers := extractHumaHandlerMap(pr.file)

	if h, ok := handlers[routeKey{"GET", "/a"}]; !ok || h != "handler" {
		t.Errorf("GET /a handler = %q", h)
	}
	if h, ok := handlers[routeKey{"POST", "/b"}]; !ok || h != "Handler" {
		t.Errorf("POST /b handler = %q", h)
	}
}

func TestAddReferencedTypes(t *testing.T) {
	pr := parseSource(t, sampleHumaService)
	structMap := map[string]*ast.StructType{}
	collectStructTypes(pr.file, structMap)

	// Create a schema that references ModelInfo
	result := &Result{
		Schemas: []Schema{
			{
				Name: "ModelsOutputBody",
				Fields: []Field{
					{Name: "models", Type: "ModelInfo", IsArray: true},
				},
			},
		},
	}
	added := map[string]bool{"ModelsOutputBody": true}

	addReferencedTypes(result, structMap, added)

	// Should have added ModelInfo schema
	if len(result.Schemas) != 2 {
		t.Fatalf("expected 2 schemas, got %d", len(result.Schemas))
	}
	mi := result.Schemas[1]
	if mi.Name != "ModelInfo" {
		t.Errorf("referenced schema name = %q", mi.Name)
	}
	if len(mi.Fields) != 3 {
		t.Errorf("ModelInfo fields = %d, want 3", len(mi.Fields))
	}
	if mi.Fields[0].Description != "Model identifier" {
		t.Errorf("ModelInfo.id description = %q", mi.Fields[0].Description)
	}
	if mi.AdditionalProperties == nil || *mi.AdditionalProperties != false {
		t.Error("expected additionalProperties = false on ModelInfo")
	}
}

func TestAddHumaErrorSchemas(t *testing.T) {
	result := &Result{}
	addHumaErrorSchemas(result)

	if result.RawSchemas == nil {
		t.Fatal("RawSchemas is nil")
	}
	if _, ok := result.RawSchemas["ErrorModel"]; !ok {
		t.Error("missing ErrorModel")
	}
	if _, ok := result.RawSchemas["ErrorDetail"]; !ok {
		t.Error("missing ErrorDetail")
	}
}

func TestExtractHumaFullIntegration(t *testing.T) {
	dir := setupProject(t, map[string]string{
		"go.mod":              "module example.com/service\nrequire github.com/danielgtaylor/huma/v2 v2.10.0\n",
		"service/handlers.go": sampleHumaService,
		"cmd/main.go": `package main

import "github.com/danielgtaylor/huma/v2"

func main() {
	cfg := huma.DefaultConfig("Runtime Service", "1.0.0")
	_ = cfg
	huma.Post(api, "/predict", Predict)
	huma.Get(api, "/models", ListModels)
	huma.Get(api, "/health", HealthCheck)
}
`,
	})

	spec, fw, err := Infer(dir, FormatJSON)
	if err != nil {
		t.Fatalf("Infer error: %v", err)
	}
	if fw != Huma {
		t.Errorf("framework = %q", fw)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(spec), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, spec)
	}

	// Check routes have operationId
	paths, _ := parsed["paths"].(map[string]any)
	if paths == nil {
		t.Fatal("no paths in spec")
	}

	// /predict should have post with operationId, requestBody, response body
	predict, _ := paths["/predict"].(map[string]any)
	post, _ := predict["post"].(map[string]any)
	if post == nil {
		t.Fatal("missing POST /predict")
	}
	if post["operationId"] != "post-predict" {
		t.Errorf("operationId = %v", post["operationId"])
	}
	if post["summary"] != "Post predict" {
		t.Errorf("summary = %v", post["summary"])
	}
	if post["requestBody"] == nil {
		t.Error("missing requestBody for POST /predict")
	}

	// Check responses
	responses, _ := post["responses"].(map[string]any)
	if responses == nil {
		t.Fatal("no responses")
	}
	ok200, _ := responses["200"].(map[string]any)
	if ok200 == nil {
		t.Fatal("no 200 response")
	}
	if ok200["content"] == nil {
		t.Error("200 response missing content")
	}
	if responses["default"] == nil {
		t.Error("missing default error response")
	}

	// /health should be GET with no requestBody
	health, _ := paths["/health"].(map[string]any)
	get, _ := health["get"].(map[string]any)
	if get == nil {
		t.Fatal("missing GET /health")
	}
	if get["operationId"] != "get-health" {
		t.Errorf("operationId = %v", get["operationId"])
	}
	if get["requestBody"] != nil {
		t.Error("GET /health should not have requestBody")
	}

	// Check schemas
	components, _ := parsed["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	if schemas == nil {
		t.Fatal("no schemas")
	}

	// PredictInputBody should exist
	pib, _ := schemas["PredictInputBody"].(map[string]any)
	if pib == nil {
		t.Fatal("missing PredictInputBody schema")
	}
	if pib["additionalProperties"] != false {
		t.Error("PredictInputBody missing additionalProperties: false")
	}

	// PredictOutputBody should exist
	if schemas["PredictOutputBody"] == nil {
		t.Error("missing PredictOutputBody schema")
	}

	// HealthOutputBody should exist
	if schemas["HealthOutputBody"] == nil {
		t.Error("missing HealthOutputBody schema")
	}

	// ModelsOutputBody should exist
	if schemas["ModelsOutputBody"] == nil {
		t.Error("missing ModelsOutputBody schema")
	}

	// ModelInfo should exist (referenced by ModelsOutputBody)
	if schemas["ModelInfo"] == nil {
		t.Error("missing ModelInfo schema")
	}

	// ErrorModel and ErrorDetail should exist
	if schemas["ErrorModel"] == nil {
		t.Error("missing ErrorModel schema")
	}
	if schemas["ErrorDetail"] == nil {
		t.Error("missing ErrorDetail schema")
	}

	// Config should NOT be in schemas (not used as API type)
	if schemas["Config"] != nil {
		t.Error("Config should not be in schemas")
	}

	// PredictInput should NOT be in schemas (only its Body is)
	if schemas["PredictInput"] != nil {
		t.Error("PredictInput wrapper should not be in schemas")
	}
}

func TestExtractHumaBodyFieldDetails(t *testing.T) {
	dir := setupProject(t, map[string]string{
		"go.mod": "module example.com/test\nrequire github.com/danielgtaylor/huma/v2 v2.10.0\n",
		"main.go": `package main

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
)

type CreateInput struct {
	Body struct {
		Data []float64 ` + "`" + `json:"data" doc:"Input data" minItems:"1"` + "`" + `
	}
}

type CreateOutput struct {
	Body struct {
		Result string ` + "`" + `json:"result" doc:"The result"` + "`" + `
	}
}

func Create(_ context.Context, input *CreateInput) (*CreateOutput, error) {
	return nil, nil
}

func main() {
	huma.Post(api, "/create", Create)
}
`,
	})

	spec, _, err := Infer(dir, FormatJSON)
	if err != nil {
		t.Fatalf("Infer error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(spec), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	components, _ := parsed["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)

	// Check CreateInputBody schema field details
	cib, _ := schemas["CreateInputBody"].(map[string]any)
	if cib == nil {
		t.Fatal("missing CreateInputBody")
	}
	props, _ := cib["properties"].(map[string]any)
	data, _ := props["data"].(map[string]any)
	if data == nil {
		t.Fatal("missing data field")
	}
	if data["description"] != "Input data" {
		t.Errorf("data description = %v", data["description"])
	}
	if data["minItems"] != float64(1) {
		t.Errorf("data minItems = %v", data["minItems"])
	}

	// type should be ["array", "null"]
	typeVal, _ := data["type"].([]any)
	if len(typeVal) != 2 || typeVal[0] != "array" || typeVal[1] != "null" {
		t.Errorf("data type = %v, want [array, null]", data["type"])
	}

	// items should have format: double
	items, _ := data["items"].(map[string]any)
	if items["format"] != "double" {
		t.Errorf("items format = %v", items["format"])
	}
	if items["type"] != "number" {
		t.Errorf("items type = %v", items["type"])
	}
}

func TestTypeNameFromExpr(t *testing.T) {
	src := `package main

import "context"

func A(_ context.Context, input *Foo) (*Bar, error) { return nil, nil }
func B(_ context.Context, _ *struct{}) (*Baz, error) { return nil, nil }
func C() {}
`
	pr := parseSource(t, src)
	sigs := parseFuncSignatures(pr.file)
	sigMap := map[string]funcSig{}
	for _, s := range sigs {
		sigMap[s.name] = s
	}

	if sig := sigMap["A"]; sig.inputType != "Foo" || sig.outputType != "Bar" {
		t.Errorf("A: input=%q output=%q", sig.inputType, sig.outputType)
	}
	if sig := sigMap["B"]; sig.inputType != "" || sig.outputType != "Baz" {
		t.Errorf("B: input=%q output=%q", sig.inputType, sig.outputType)
	}
	if _, ok := sigMap["C"]; ok {
		t.Error("C should not have a signature (no input/output types)")
	}
}
