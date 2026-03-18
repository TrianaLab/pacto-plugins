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

func detectHuma(dir string) bool {
	return fileContains(dir, "go.mod", "danielgtaylor/huma")
}

func extractHuma(dir string) (*Result, error) {
	result := &Result{Framework: Huma}
	fset := token.NewFileSet()

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
		result.Schemas = append(result.Schemas, parseGoStructsAST(file)...)
	}

	return result, nil
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

// --- Go struct schemas ---

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
