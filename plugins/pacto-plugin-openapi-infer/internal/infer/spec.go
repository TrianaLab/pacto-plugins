package infer

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// OutputFormat controls the serialization of the OpenAPI spec.
type OutputFormat int

const (
	FormatYAML OutputFormat = iota
	FormatJSON
)

// AppInfo holds API metadata.
type AppInfo struct {
	Title   string
	Version string
}

// Param represents a route parameter.
type Param struct {
	Name     string
	In       string // path, query
	Type     string // string, integer, number, boolean
	Required bool
}

// BodyRef references a schema type for request/response bodies.
type BodyRef struct {
	Name    string
	IsArray bool
}

// Route represents an extracted HTTP route.
type Route struct {
	Method  string // GET, POST, PUT, DELETE, PATCH
	Path    string // OpenAPI path: /users/{id}
	Summary string
	Params  []Param
	ReqBody *BodyRef
	ResBody *BodyRef
}

// Field represents a property in a schema.
type Field struct {
	Name    string
	Type    string // OpenAPI type or schema name
	IsArray bool
}

// Schema represents an extracted data model.
type Schema struct {
	Name   string
	Fields []Field
}

// Result holds all extracted API information.
type Result struct {
	Framework Framework
	Info      AppInfo
	Routes    []Route
	Schemas   []Schema
}

// BuildSpec constructs an OpenAPI 3.1.0 spec from a Result in the given format.
func BuildSpec(r *Result, format ...OutputFormat) (string, error) {
	outFmt := FormatYAML
	if len(format) > 0 {
		outFmt = format[0]
	}
	if len(r.Routes) == 0 {
		return "", fmt.Errorf("no routes found")
	}

	title := r.Info.Title
	if title == "" {
		title = "API"
	}
	version := r.Info.Version
	if version == "" {
		version = "0.0.0"
	}

	spec := map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":   title,
			"version": version,
		},
	}

	paths := buildPaths(r.Routes)
	spec["paths"] = paths

	if len(r.Schemas) > 0 {
		spec["components"] = map[string]any{
			"schemas": buildSchemas(r.Schemas),
		}
	}

	var out []byte
	var err error
	if outFmt == FormatJSON {
		out, err = json.MarshalIndent(spec, "", "  ")
	} else {
		out, err = yaml.Marshal(spec)
	}
	if err != nil {
		return "", fmt.Errorf("failed to marshal OpenAPI spec: %w", err)
	}
	return string(out), nil
}

func buildPaths(routes []Route) map[string]any {
	paths := map[string]any{}

	for _, route := range routes {
		if _, ok := paths[route.Path]; !ok {
			paths[route.Path] = map[string]any{}
		}

		op := map[string]any{}
		if route.Summary != "" {
			op["summary"] = route.Summary
		}

		if len(route.Params) > 0 {
			var params []any
			for _, p := range route.Params {
				params = append(params, map[string]any{
					"name":     p.Name,
					"in":       p.In,
					"required": p.Required,
					"schema":   map[string]any{"type": p.Type},
				})
			}
			op["parameters"] = params
		}

		if route.ReqBody != nil {
			schema := refOrType(route.ReqBody.Name)
			if route.ReqBody.IsArray {
				schema = map[string]any{"type": "array", "items": schema}
			}
			op["requestBody"] = map[string]any{
				"required": true,
				"content": map[string]any{
					"application/json": map[string]any{"schema": schema},
				},
			}
		}

		resp := map[string]any{"description": "Successful response"}
		if route.ResBody != nil {
			schema := refOrType(route.ResBody.Name)
			if route.ResBody.IsArray {
				schema = map[string]any{"type": "array", "items": schema}
			}
			resp["content"] = map[string]any{
				"application/json": map[string]any{"schema": schema},
			}
		}
		op["responses"] = map[string]any{"200": resp}

		paths[route.Path].(map[string]any)[strings.ToLower(route.Method)] = op
	}

	return paths
}

func buildSchemas(schemas []Schema) map[string]any {
	result := map[string]any{}

	for _, s := range schemas {
		props := map[string]any{}
		var required []string

		for _, f := range s.Fields {
			if f.IsArray {
				props[f.Name] = map[string]any{
					"type":  "array",
					"items": refOrType(f.Type),
				}
			} else {
				props[f.Name] = refOrType(f.Type)
			}
			required = append(required, f.Name)
		}

		schema := map[string]any{
			"type":       "object",
			"properties": props,
		}
		if len(required) > 0 {
			schema["required"] = required
		}
		result[s.Name] = schema
	}

	return result
}

func refOrType(name string) map[string]any {
	if isPrimitive(name) {
		return map[string]any{"type": name}
	}
	return map[string]any{"$ref": "#/components/schemas/" + name}
}

// marshalYAML converts an arbitrary value to a YAML string.
func marshalYAML(v any) (string, error) {
	out, err := yaml.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("failed to marshal YAML: %w", err)
	}
	return string(out), nil
}
