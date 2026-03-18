package infer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strconv"
	"strings"
)

var goTypeMap = map[string]string{
	"string": "string",
	"int":    "integer", "int8": "integer", "int16": "integer",
	"int32": "integer", "int64": "integer",
	"uint": "integer", "uint8": "integer", "uint16": "integer",
	"uint32": "integer", "uint64": "integer",
	"float32": "number", "float64": "number",
	"bool": "boolean",
}

var goFormatMap = map[string]string{
	"float64": "double",
	"float32": "float",
	"int64":   "int64",
	"int32":   "int32",
}

// routeKey uniquely identifies a route by method and path.
type routeKey struct {
	method string
	path   string
}

// funcSig holds the input/output type names from a function signature.
type funcSig struct {
	name       string
	inputType  string
	outputType string
}

func detectHuma(dir string) bool {
	return fileContains(dir, "go.mod", "danielgtaylor/huma")
}

func extractHuma(dir string) (*Result, error) {
	result := &Result{Framework: Huma}
	fset := token.NewFileSet()

	funcSigs := map[string]funcSig{}
	handlerMap := map[routeKey]string{}
	structMap := map[string]*ast.StructType{}

	for _, f := range findFiles(dir, ".go") {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		file, err := parser.ParseFile(fset, f, data, 0)
		if err != nil {
			continue
		}

		if info, ok := parseHumaInfoAST(file); ok && result.Info.Title == "" {
			result.Info = info
		}
		result.Routes = append(result.Routes, parseHumaRoutesAST(file)...)

		for k, v := range extractHumaHandlerMap(file) {
			handlerMap[k] = v
		}
		for _, sig := range parseFuncSignatures(file) {
			funcSigs[sig.name] = sig
		}
		collectStructTypes(file, structMap)
	}

	// Link routes to handler types and enrich with Huma conventions.
	addedSchemas := enrichHumaRoutes(result, handlerMap, funcSigs, structMap)

	// Add schemas for types referenced by fields (e.g., ModelInfo).
	addReferencedTypes(result, structMap, addedSchemas)

	// Add standard Huma error schemas.
	addHumaErrorSchemas(result)

	return result, nil
}

// enrichHumaRoutes adds operationId, summary, default error refs, and links
// routes to their handler input/output Body schemas.
func enrichHumaRoutes(result *Result, handlerMap map[routeKey]string, funcSigs map[string]funcSig, structMap map[string]*ast.StructType) map[string]bool {
	addedSchemas := map[string]bool{}
	for i := range result.Routes {
		route := &result.Routes[i]

		route.OperationId = generateOperationId(route.Method, route.Path)
		if route.Summary == "" {
			route.Summary = generateSummary(route.Method, route.Path)
		}
		route.DefaultErrorRef = "ErrorModel"

		key := routeKey{route.Method, route.Path}
		handlerName, ok := handlerMap[key]
		if !ok {
			continue
		}
		sig, ok := funcSigs[handlerName]
		if !ok {
			continue
		}

		if sig.inputType != "" {
			if bodyName := processBodyType(sig.inputType, structMap, result, addedSchemas); bodyName != "" {
				route.ReqBody = &BodyRef{Name: bodyName}
			}
		}
		if sig.outputType != "" {
			if bodyName := processBodyType(sig.outputType, structMap, result, addedSchemas); bodyName != "" {
				route.ResBody = &BodyRef{Name: bodyName}
			}
		}
	}
	return addedSchemas
}

// --- App info ---

func parseHumaInfoAST(file *ast.File) (AppInfo, bool) {
	var info AppInfo
	var found bool

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok || ident.Name != "huma" || sel.Sel.Name != "DefaultConfig" {
			return true
		}
		if len(call.Args) >= 2 {
			info.Title = stringLitValue(call.Args[0])
			info.Version = stringLitValue(call.Args[1])
			found = true
		}
		return false
	})

	return info, found
}

// --- Routes ---

var humaMethodFuncs = map[string]string{
	"Get": "GET", "Post": "POST", "Put": "PUT",
	"Delete": "DELETE", "Patch": "PATCH",
}

func parseHumaRoutesAST(file *ast.File) []Route {
	var routes []Route

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok || ident.Name != "huma" {
			return true
		}

		// huma.Get/Post/Put/Delete/Patch shorthand.
		if method, ok := humaMethodFuncs[sel.Sel.Name]; ok {
			if len(call.Args) >= 2 {
				if path := stringLitValue(call.Args[1]); path != "" {
					routes = append(routes, makeHumaRoute(method, path, ""))
				}
			}
			return true
		}

		// huma.Register with Operation struct.
		if sel.Sel.Name == "Register" && len(call.Args) >= 2 {
			if route, ok := parseHumaOperation(call.Args[1]); ok {
				routes = append(routes, route)
			}
		}

		return true
	})

	return routes
}

func parseHumaOperation(expr ast.Expr) (Route, bool) {
	comp, ok := expr.(*ast.CompositeLit)
	if !ok {
		return Route{}, false
	}

	var method, path, summary string
	for _, elt := range comp.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "Method":
			method = resolveHTTPMethod(kv.Value)
		case "Path":
			path = stringLitValue(kv.Value)
		case "Summary":
			summary = stringLitValue(kv.Value)
		}
	}

	if method == "" || path == "" {
		return Route{}, false
	}
	return makeHumaRoute(method, path, summary), true
}

func resolveHTTPMethod(expr ast.Expr) string {
	if s := stringLitValue(expr); s != "" {
		return strings.ToUpper(s)
	}
	// http.MethodGet → "GET"
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if method, ok := strings.CutPrefix(sel.Sel.Name, "Method"); ok {
			return strings.ToUpper(method)
		}
	}
	return ""
}

func makeHumaRoute(method, path, summary string) Route {
	var params []Param
	for _, pp := range pathParams(path) {
		params = append(params, Param{
			Name: pp, In: "path", Type: "string", Required: true,
		})
	}
	return Route{
		Method:  method,
		Path:    path,
		Summary: summary,
		Params:  params,
	}
}

// --- Handler map extraction ---

// extractHumaHandlerMap extracts a mapping of {method, path} → handler function name.
func extractHumaHandlerMap(file *ast.File) map[routeKey]string {
	handlers := map[routeKey]string{}

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok || ident.Name != "huma" {
			return true
		}

		if method, ok := humaMethodFuncs[sel.Sel.Name]; ok {
			if len(call.Args) >= 3 {
				path := stringLitValue(call.Args[1])
				name := funcNameFromExpr(call.Args[2])
				if path != "" && name != "" {
					handlers[routeKey{method, path}] = name
				}
			}
		}

		if sel.Sel.Name == "Register" && len(call.Args) >= 3 {
			if route, ok := parseHumaOperation(call.Args[1]); ok {
				name := funcNameFromExpr(call.Args[2])
				if name != "" {
					handlers[routeKey{route.Method, route.Path}] = name
				}
			}
		}

		return true
	})

	return handlers
}

// funcNameFromExpr extracts the function name from an expression like
// `Predict` or `pkg.Predict`.
func funcNameFromExpr(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	}
	return ""
}

// --- Function signature parsing ---

// parseFuncSignatures extracts function names and their input/output types.
func parseFuncSignatures(file *ast.File) []funcSig {
	var sigs []funcSig
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Type.Params == nil {
			continue
		}
		sig := funcSig{name: fd.Name.Name}

		// Second parameter (after context.Context) is the input type.
		params := fd.Type.Params.List
		if len(params) >= 2 {
			sig.inputType = typeNameFromExpr(params[1].Type)
		}

		// First return type is the output type.
		if fd.Type.Results != nil && len(fd.Type.Results.List) >= 1 {
			sig.outputType = typeNameFromExpr(fd.Type.Results.List[0].Type)
		}

		if sig.inputType != "" || sig.outputType != "" {
			sigs = append(sigs, sig)
		}
	}
	return sigs
}

// typeNameFromExpr extracts a type name, unwrapping pointers.
// Returns "" for anonymous types like struct{}.
func typeNameFromExpr(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return typeNameFromExpr(e.X)
	case *ast.SelectorExpr:
		if ident, ok := e.X.(*ast.Ident); ok {
			return ident.Name + "." + e.Sel.Name
		}
	}
	return ""
}

// --- Struct type collection ---

// collectStructTypes gathers all named struct type definitions.
func collectStructTypes(file *ast.File, m map[string]*ast.StructType) {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if st, ok := typeSpec.Type.(*ast.StructType); ok {
				m[typeSpec.Name.Name] = st
			}
		}
	}
}

// --- Body type processing ---

// processBodyType handles the Huma Body struct convention. If the named struct
// has a Body field that is an anonymous struct, it creates a "{Name}Body" schema
// with the inner fields. Returns the schema name, or "" if not applicable.
func processBodyType(typeName string, structMap map[string]*ast.StructType, result *Result, added map[string]bool) string {
	st, ok := structMap[typeName]
	if !ok {
		return ""
	}

	for _, field := range st.Fields.List {
		if len(field.Names) == 0 || field.Names[0].Name != "Body" {
			continue
		}
		bodyStruct, ok := field.Type.(*ast.StructType)
		if !ok {
			continue
		}

		bodyName := typeName + "Body"
		if added[bodyName] {
			return bodyName
		}

		schema := Schema{
			Name:                 bodyName,
			AdditionalProperties: boolPtr(false),
		}
		for _, bf := range bodyStruct.Fields.List {
			if f, ok := parseHumaStructField(bf); ok {
				schema.Fields = append(schema.Fields, f)
			}
		}

		result.Schemas = append(result.Schemas, schema)
		added[bodyName] = true
		return bodyName
	}

	return ""
}

// parseHumaStructField parses a struct field with Huma-specific conventions:
// doc tags for descriptions, minItems tags, format mapping, nullable arrays.
func parseHumaStructField(field *ast.Field) (Field, bool) {
	if len(field.Names) == 0 {
		return Field{}, false
	}
	fieldName := field.Names[0].Name
	if !ast.IsExported(fieldName) {
		return Field{}, false
	}

	jsonName := fieldName
	var description string
	var minItems int

	if field.Tag != nil {
		tag := strings.Trim(field.Tag.Value, "`")
		st := reflect.StructTag(tag)

		if jn := st.Get("json"); jn != "" {
			parts := strings.SplitN(jn, ",", 2)
			if parts[0] == "-" {
				return Field{}, false
			}
			if parts[0] != "" {
				jsonName = parts[0]
			}
		}

		description = st.Get("doc")
		if mi := st.Get("minItems"); mi != "" {
			minItems, _ = strconv.Atoi(mi)
		}
	}

	goType, isArray := resolveGoFieldType(field.Type)
	oaType := mapGoType(goType)
	format := goFormatMap[goType]

	return Field{
		Name:          jsonName,
		Type:          oaType,
		IsArray:       isArray,
		Description:   description,
		Format:        format,
		MinItems:      minItems,
		NullableArray: isArray, // Huma convention: slices are nullable
	}, true
}

// addReferencedTypes adds schemas for struct types referenced by schema fields
// (e.g., ModelInfo used as an array item type).
func addReferencedTypes(result *Result, structMap map[string]*ast.StructType, added map[string]bool) {
	for i := 0; i < len(result.Schemas); i++ {
		for _, f := range result.Schemas[i].Fields {
			typeName := f.Type
			if isPrimitive(typeName) || added[typeName] {
				continue
			}
			st, ok := structMap[typeName]
			if !ok {
				continue
			}

			schema := Schema{
				Name:                 typeName,
				AdditionalProperties: boolPtr(false),
			}
			for _, sf := range st.Fields.List {
				if field, ok := parseHumaStructField(sf); ok {
					schema.Fields = append(schema.Fields, field)
				}
			}
			if len(schema.Fields) > 0 {
				result.Schemas = append(result.Schemas, schema)
				added[typeName] = true
			}
		}
	}
}

// --- Huma error schemas ---

func addHumaErrorSchemas(result *Result) {
	if result.RawSchemas == nil {
		result.RawSchemas = map[string]any{}
	}

	result.RawSchemas["ErrorDetail"] = map[string]any{
		"additionalProperties": false,
		"type":                 "object",
		"properties": map[string]any{
			"location": map[string]any{
				"description": "Where the error occurred, e.g. 'body.items[3].tags' or 'path.thing-id'",
				"type":        "string",
			},
			"message": map[string]any{
				"description": "Error message text",
				"type":        "string",
			},
			"value": map[string]any{
				"description": "The value at the given location",
			},
		},
	}

	result.RawSchemas["ErrorModel"] = map[string]any{
		"additionalProperties": false,
		"type":                 "object",
		"properties": map[string]any{
			"$schema": map[string]any{
				"description": "A URL to the JSON Schema for this object.",
				"examples":    []any{"https://example.com/schemas/ErrorModel.json"},
				"format":      "uri",
				"readOnly":    true,
				"type":        "string",
			},
			"detail": map[string]any{
				"description": "A human-readable explanation specific to this occurrence of the problem.",
				"examples":    []any{"Property foo is required but is missing."},
				"type":        "string",
			},
			"errors": map[string]any{
				"description": "Optional list of individual error details",
				"items":       map[string]any{"$ref": "#/components/schemas/ErrorDetail"},
				"type":        []any{"array", "null"},
			},
			"instance": map[string]any{
				"description": "A URI reference that identifies the specific occurrence of the problem.",
				"examples":    []any{"https://example.com/error-log/abc123"},
				"format":      "uri",
				"type":        "string",
			},
			"status": map[string]any{
				"description": "HTTP status code",
				"examples":    []any{400},
				"format":      "int64",
				"type":        "integer",
			},
			"title": map[string]any{
				"description": "A short, human-readable summary of the problem type.",
				"examples":    []any{"Bad Request"},
				"type":        "string",
			},
			"type": map[string]any{
				"default":     "about:blank",
				"description": "A URI reference to human-readable documentation for the error.",
				"examples":    []any{"https://example.com/errors/example"},
				"format":      "uri",
				"type":        "string",
			},
		},
	}
}

// --- Operation ID and summary generation ---

func generateOperationId(method, path string) string {
	path = strings.NewReplacer("{", "", "}", "").Replace(path)
	segments := strings.Split(strings.Trim(path, "/"), "/")
	return strings.ToLower(method) + "-" + strings.Join(segments, "-")
}

func generateSummary(method, path string) string {
	path = strings.NewReplacer("{", "", "}", "").Replace(path)
	segments := strings.Split(strings.Trim(path, "/"), "/")
	m := strings.ToUpper(method[:1]) + strings.ToLower(method[1:])
	return m + " " + strings.Join(segments, " ")
}

// --- Go struct schemas (legacy, used by tests) ---

func parseGoStructsAST(file *ast.File) []Schema {
	var schemas []Schema

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			name := typeSpec.Name.Name
			if !ast.IsExported(name) {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			schema := Schema{Name: name}
			for _, field := range structType.Fields.List {
				if f, ok := parseStructField(field); ok {
					schema.Fields = append(schema.Fields, f)
				}
			}

			if len(schema.Fields) > 0 {
				schemas = append(schemas, schema)
			}
		}
	}

	return schemas
}

func parseStructField(field *ast.Field) (Field, bool) {
	if len(field.Names) == 0 {
		return Field{}, false
	}
	fieldName := field.Names[0].Name
	if !ast.IsExported(fieldName) {
		return Field{}, false
	}

	jsonName := fieldName
	if field.Tag != nil {
		tag := strings.Trim(field.Tag.Value, "`")
		if jn := reflect.StructTag(tag).Get("json"); jn != "" {
			parts := strings.SplitN(jn, ",", 2)
			if parts[0] == "-" {
				return Field{}, false
			}
			if parts[0] != "" {
				jsonName = parts[0]
			}
		}
	}

	fieldType, isArray := resolveGoFieldType(field.Type)
	oaType := mapGoType(fieldType)
	return Field{Name: jsonName, Type: oaType, IsArray: isArray}, true
}

func resolveGoFieldType(expr ast.Expr) (string, bool) {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name, false
	case *ast.ArrayType:
		inner, _ := resolveGoFieldType(t.Elt)
		return inner, true
	case *ast.StarExpr:
		return resolveGoFieldType(t.X)
	case *ast.SelectorExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name + "." + t.Sel.Name, false
		}
	}
	return "string", false
}

func mapGoType(t string) string {
	if mapped, ok := goTypeMap[t]; ok {
		return mapped
	}
	return t
}

// --- Helpers ---

func stringLitValue(expr ast.Expr) string {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return ""
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return strings.Trim(lit.Value, "`\"")
	}
	return s
}
