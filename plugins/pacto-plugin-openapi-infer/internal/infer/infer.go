// Package infer auto-detects web frameworks and extracts OpenAPI 3.1
// specs from source code. FastAPI uses runtime extraction via the
// project's Python environment; Huma uses Go AST-based static analysis.
package infer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/trianalab/pacto-plugins/plugins/pacto-plugin-openapi-infer/internal/runtime"
)

// Framework represents a supported web framework.
type Framework string

const (
	FastAPI Framework = "fastapi"
	Huma    Framework = "huma"
)

type handler struct {
	framework Framework
	detect    func(string) bool
	extract   func(string) (string, error) // returns JSON spec
}

var handlers = []handler{
	{FastAPI, detectFastAPI, extractFastAPIRuntime},
	{Huma, detectHuma, extractHumaJSON},
}

// Infer detects the framework and extracts an OpenAPI spec.
// Defaults to YAML; pass FormatJSON for JSON output.
func Infer(dir string, format ...OutputFormat) (string, Framework, error) {
	for _, h := range handlers {
		if h.detect(dir) {
			return runHandler(h, dir, format...)
		}
	}
	return "", "", fmt.Errorf(
		"no supported framework detected\nsupported: fastapi, huma",
	)
}

// InferWithFramework extracts using a specific framework (skips detection).
func InferWithFramework(bundleDir string, fw Framework, format ...OutputFormat) (string, error) {
	for _, h := range handlers {
		if h.framework == fw {
			spec, _, err := runHandler(h, bundleDir, format...)
			return spec, err
		}
	}
	return "", fmt.Errorf("unsupported framework: %s", fw)
}

func runHandler(h handler, bundleDir string, format ...OutputFormat) (string, Framework, error) {
	jsonSpec, err := h.extract(bundleDir)
	if err != nil {
		return "", h.framework, fmt.Errorf("%s: %w", h.framework, err)
	}
	formatted, err := reformatSpec(jsonSpec, format...)
	if err != nil {
		return "", h.framework, err
	}
	return formatted, h.framework, nil
}

// extractFastAPIRuntime extracts the OpenAPI spec by running the FastAPI
// app using the project's Python environment.
func extractFastAPIRuntime(sourceDir string) (string, error) {
	spec, err := runtime.ExtractFastAPI(sourceDir, "")
	if err != nil {
		return "", fmt.Errorf(
			"runtime extraction failed: %w\nensure the project's Python environment is set up with all dependencies installed",
			err,
		)
	}
	return spec, nil
}

// extractHumaJSON extracts the spec via Go AST analysis and returns JSON.
func extractHumaJSON(dir string) (string, error) {
	result, err := extractHuma(dir)
	if err != nil {
		return "", err
	}
	return BuildSpec(result, FormatJSON)
}

// reformatSpec takes a JSON spec string and converts it to the requested format.
func reformatSpec(jsonSpec string, format ...OutputFormat) (string, error) {
	outFmt := FormatYAML
	if len(format) > 0 {
		outFmt = format[0]
	}

	var obj any
	if err := json.Unmarshal([]byte(jsonSpec), &obj); err != nil {
		return "", fmt.Errorf("invalid JSON spec: %w", err)
	}

	if outFmt == FormatJSON {
		out, err := json.MarshalIndent(obj, "", "  ")
		if err != nil {
			return "", err
		}
		return string(out), nil
	}

	return marshalYAML(obj)
}

// --- Common helpers ---

var skipDirs = map[string]bool{
	".git": true, "vendor": true, "node_modules": true,
	"__pycache__": true, "target": true, "build": true,
	"dist": true, ".venv": true, "venv": true, "env": true,
	".tox": true, ".mypy_cache": true, ".pytest_cache": true,
	".gradle": true, ".idea": true, ".vscode": true,
}

func findFiles(dir, ext string) []string {
	var files []string
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) == ext {
			files = append(files, path)
		}
		return nil
	})
	return files
}

func fileContains(dir, filename, substr string) bool {
	data, err := os.ReadFile(filepath.Join(dir, filename))
	if err != nil {
		return false
	}
	return strings.Contains(string(data), substr)
}

var pathParamCurlyRe = regexp.MustCompile(`\{(\w[\w-]*)\}`)

func pathParams(path string) []string {
	matches := pathParamCurlyRe.FindAllStringSubmatch(path, -1)
	params := make([]string, len(matches))
	for i, m := range matches {
		params[i] = m[1]
	}
	return params
}

func isPrimitive(t string) bool {
	switch t {
	case "string", "integer", "number", "boolean":
		return true
	}
	return false
}
