// pacto-plugin-openapi-infer auto-detects web frameworks and extracts
// OpenAPI specs from source code, following the Pacto plugin protocol.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/trianalab/pacto-plugins/plugins/pacto-plugin-openapi-infer/internal/infer"
)

// version is set at build time via ldflags.
var version = "dev"

type generateRequest struct {
	BundleDir string         `json:"bundleDir"`
	Options   map[string]any `json:"options,omitempty"`
}

type generatedFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type generateResponse struct {
	Files   []generatedFile `json:"files"`
	Message string          `json:"message,omitempty"`
}

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-version" || os.Args[1] == "--version") {
		fmt.Println(version)
		return
	}

	var req generateRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		fmt.Fprintf(os.Stderr, "failed to decode input: %v", err)
		os.Exit(1)
	}

	output, _ := req.Options["output"].(string)
	if output == "" {
		output = "interfaces/openapi.yaml"
	}

	// Detect output format from file extension.
	format := infer.FormatYAML
	if strings.ToLower(filepath.Ext(output)) == ".json" {
		format = infer.FormatJSON
	}

	var spec string
	var fw infer.Framework
	var err error

	// Allow explicit framework override via --option framework=X.
	if fwStr, _ := req.Options["framework"].(string); fwStr != "" {
		fw = infer.Framework(strings.ToLower(fwStr))
		spec, err = infer.InferWithFramework(req.BundleDir, fw, format)
	} else {
		spec, fw, err = infer.Infer(req.BundleDir, format)
	}

	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}

	resp := generateResponse{
		Files: []generatedFile{
			{Path: output, Content: spec},
		},
		Message: fmt.Sprintf(
			"Inferred OpenAPI spec from %s project — add interfaces[].contract: %s to your pacto.yaml",
			fw, output,
		),
	}

	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		fmt.Fprintf(os.Stderr, "failed to encode output: %v", err)
		os.Exit(1)
	}
}
