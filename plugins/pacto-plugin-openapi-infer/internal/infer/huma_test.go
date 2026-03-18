package infer

import (
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
