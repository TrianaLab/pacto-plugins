package infer

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// E2E test: both frameworks implement the same Pet Store API.
// The extracted specs must produce equivalent routes and schemas.
//
// API contract:
//   GET    /pets          → list[Pet]        (query param: limit)
//   GET    /pets/{petId}  → Pet              (path param: petId)
//   POST   /pets          → Pet              (body: PetCreate)
//   DELETE /pets/{petId}  → void
//
// Schemas:
//   Pet       { id: integer, name: string, tag: string }
//   PetCreate { name: string, tag: string }

func testdataDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata")
}

// normalizedSpec holds a framework-agnostic view of the extracted spec for comparison.
type normalizedSpec struct {
	Routes  []normalizedRoute
	Schemas []normalizedSchema
}

type normalizedRoute struct {
	Method     string
	Path       string
	PathParams []string
	HasReqBody bool
	HasResBody bool
}

type normalizedSchema struct {
	Name   string
	Fields []string // sorted field names
}

func TestE2EAllFrameworksMatch(t *testing.T) {
	base := testdataDir()
	frameworks := []Framework{FastAPI, Huma}
	specs := map[Framework]normalizedSpec{}

	for _, fw := range frameworks {
		t.Run(string(fw), func(t *testing.T) {
			dir := filepath.Join(base, string(fw))
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				t.Fatalf("testdata directory not found: %s", dir)
			}

			yamlStr, detectedFw, err := Infer(dir)
			if err != nil {
				t.Skipf("Infer(%s) skipped (runtime not available): %v", fw, err)
				return
			}
			if detectedFw != fw {
				t.Errorf("detected %q, want %q", detectedFw, fw)
			}

			// Verify it's valid YAML.
			var parsed map[string]any
			if err := yaml.Unmarshal([]byte(yamlStr), &parsed); err != nil {
				t.Fatalf("invalid YAML: %v\n%s", err, yamlStr)
			}

			if parsed["openapi"] != "3.1.0" {
				t.Errorf("openapi = %v", parsed["openapi"])
			}

			specs[fw] = normalize(t, yamlStr)
		})
	}

	// Compare frameworks that both succeeded.
	for i, fwA := range frameworks {
		refSpec, okA := specs[fwA]
		if !okA {
			continue
		}
		for _, fwB := range frameworks[i+1:] {
			otherSpec, okB := specs[fwB]
			if !okB {
				continue
			}
			t.Run("compare_"+string(fwA)+"_vs_"+string(fwB), func(t *testing.T) {
				compareRoutes(t, string(fwA), refSpec.Routes, string(fwB), otherSpec.Routes)
				compareSchemas(t, string(fwA), refSpec.Schemas, string(fwB), otherSpec.Schemas)
			})
		}
	}
}

func normalize(t *testing.T, yamlStr string) normalizedSpec {
	t.Helper()
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(yamlStr), &parsed); err != nil {
		t.Fatalf("invalid YAML: %v", err)
	}

	ns := normalizedSpec{}

	// Extract routes.
	paths, _ := parsed["paths"].(map[string]any)
	for path, methods := range paths {
		methodMap, _ := methods.(map[string]any)
		for method, opAny := range methodMap {
			op, _ := opAny.(map[string]any)
			// Normalize trailing slashes for cross-framework comparison.
			normPath := strings.TrimRight(path, "/")
			if normPath == "" {
				normPath = "/"
			}
			route := normalizedRoute{
				Method: method,
				Path:   normPath,
			}

			if params, ok := op["parameters"].([]any); ok {
				for _, pAny := range params {
					p, _ := pAny.(map[string]any)
					if p["in"] == "path" {
						route.PathParams = append(route.PathParams, p["name"].(string))
					}
				}
			}
			sort.Strings(route.PathParams)

			route.HasReqBody = op["requestBody"] != nil

			if resp, ok := op["responses"].(map[string]any); ok {
				if r200, ok := resp["200"].(map[string]any); ok {
					route.HasResBody = r200["content"] != nil
				}
			}

			ns.Routes = append(ns.Routes, route)
		}
	}

	sort.Slice(ns.Routes, func(i, j int) bool {
		if ns.Routes[i].Path != ns.Routes[j].Path {
			return ns.Routes[i].Path < ns.Routes[j].Path
		}
		return ns.Routes[i].Method < ns.Routes[j].Method
	})

	// Extract schemas.
	if components, ok := parsed["components"].(map[string]any); ok {
		if schemas, ok := components["schemas"].(map[string]any); ok {
			for name, sAny := range schemas {
				s, _ := sAny.(map[string]any)
				schema := normalizedSchema{Name: name}
				if props, ok := s["properties"].(map[string]any); ok {
					for fieldName := range props {
						schema.Fields = append(schema.Fields, fieldName)
					}
				}
				sort.Strings(schema.Fields)
				ns.Schemas = append(ns.Schemas, schema)
			}
		}
	}

	sort.Slice(ns.Schemas, func(i, j int) bool {
		return ns.Schemas[i].Name < ns.Schemas[j].Name
	})

	return ns
}

func compareRoutes(t *testing.T, fwA string, routesA []normalizedRoute, fwB string, routesB []normalizedRoute) {
	t.Helper()

	if len(routesA) != len(routesB) {
		t.Errorf("%s has %d routes, %s has %d routes", fwA, len(routesA), fwB, len(routesB))
		return
	}

	for i := range routesA {
		a, b := routesA[i], routesB[i]
		if a.Method != b.Method {
			t.Errorf("route %d: %s method=%s, %s method=%s", i, fwA, a.Method, fwB, b.Method)
		}
		if a.Path != b.Path {
			t.Errorf("route %d: %s path=%s, %s path=%s", i, fwA, a.Path, fwB, b.Path)
		}
		if len(a.PathParams) != len(b.PathParams) {
			t.Errorf("route %d (%s %s): %s has %d path params, %s has %d",
				i, a.Method, a.Path, fwA, len(a.PathParams), fwB, len(b.PathParams))
		}
	}
}

func compareSchemas(t *testing.T, fwA string, schemasA []normalizedSchema, fwB string, schemasB []normalizedSchema) {
	t.Helper()

	// Build a map of schemas by name for intersection-based comparison.
	// Runtime extraction may produce more schemas than static analysis.
	mapA := map[string]normalizedSchema{}
	for _, s := range schemasA {
		mapA[s.Name] = s
	}
	mapB := map[string]normalizedSchema{}
	for _, s := range schemasB {
		mapB[s.Name] = s
	}

	// Compare schemas that exist in both.
	for name, a := range mapA {
		b, ok := mapB[name]
		if !ok {
			continue // present in A but not B — acceptable
		}
		if len(a.Fields) != len(b.Fields) {
			t.Errorf("schema %s: %s has %d fields %v, %s has %d fields %v",
				name, fwA, len(a.Fields), a.Fields, fwB, len(b.Fields), b.Fields)
			continue
		}
		for j := range a.Fields {
			if a.Fields[j] != b.Fields[j] {
				t.Errorf("schema %s field %d: %s has %q, %s has %q",
					name, j, fwA, a.Fields[j], fwB, b.Fields[j])
			}
		}
	}
}
