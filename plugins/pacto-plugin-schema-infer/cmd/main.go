// pacto-plugin-schema-infer reads a service configuration file and infers
// a JSON Schema from it, following the Pacto plugin protocol.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"

	"github.com/trianalab/pacto-plugins/plugins/pacto-plugin-schema-infer/internal/infer"
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

	filename, _ := req.Options["file"].(string)
	if filename == "" {
		fmt.Fprint(os.Stderr, "missing required option: file (e.g. --option file=config.yaml)")
		os.Exit(1)
	}

	filePath := filepath.Join(req.BundleDir, filename)
	raw, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read file %s: %v", filename, err)
		os.Exit(1)
	}

	data, err := parseFile(filename, raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse %s: %v", filename, err)
		os.Exit(1)
	}

	schema := infer.Schema(data)
	title, _ := req.Options["title"].(string)
	if title == "" {
		title = "configuration"
	}
	schema["title"] = title

	schemaJSON, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal schema: %v", err)
		os.Exit(1)
	}

	resp := generateResponse{
		Files: []generatedFile{
			{
				Path:    "config.schema.json",
				Content: string(schemaJSON) + "\n",
			},
		},
		Message: fmt.Sprintf(
			"Inferred JSON Schema from %s — add configuration.schema: config.schema.json to your pacto.yaml to use it",
			filename,
		),
	}

	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		fmt.Fprintf(os.Stderr, "failed to encode output: %v", err)
		os.Exit(1)
	}
}

func parseFile(filename string, raw []byte) (map[string]any, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	var data map[string]any

	switch ext {
	case ".json":
		if err := json.Unmarshal(raw, &data); err != nil {
			return nil, err
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(raw, &data); err != nil {
			return nil, err
		}
	case ".toml":
		if err := toml.Unmarshal(raw, &data); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported file extension: %s", ext)
	}

	return data, nil
}
