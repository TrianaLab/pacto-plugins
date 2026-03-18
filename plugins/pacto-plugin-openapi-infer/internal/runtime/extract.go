// Package runtime extracts OpenAPI specs by running the actual
// framework code using the project's Python environment.
package runtime

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed scripts/extract_fastapi.py
var extractFastAPIScript string

// ExtractFastAPI runs the FastAPI app and returns the OpenAPI spec as JSON.
// It finds the project's Python (virtualenv, uv, system) and runs the
// extraction script with all dependencies available.
// appSpec is optional — e.g. "myapp.main:app". If empty, auto-discovers.
func ExtractFastAPI(sourceDir string, appSpec string) (string, error) {
	pythonPath, err := findProjectPython(sourceDir)
	if err != nil {
		return "", err
	}

	return runPythonExtract(pythonPath, sourceDir, appSpec)
}

// findProjectPython locates the Python interpreter for the project.
// It searches for virtualenvs near the source dir, then falls back
// to uv run, then system Python.
func findProjectPython(sourceDir string) (string, error) {
	// Walk up from sourceDir looking for .venv/bin/python or venv/bin/python
	dir := sourceDir
	for i := 0; i < 10; i++ {
		for _, venvDir := range []string{".venv", "venv", "env"} {
			candidate := filepath.Join(dir, venvDir, "bin", "python")
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}

		// Check for uv project (pyproject.toml with uv)
		if _, err := os.Stat(filepath.Join(dir, "pyproject.toml")); err == nil {
			if uvPath, err := exec.LookPath("uv"); err == nil {
				// uv run python will use the project's virtualenv
				return uvPath, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Fall back to system Python
	for _, name := range []string{"python3", "python"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("no Python interpreter found (checked virtualenvs and system PATH)")
}

// findProjectRoot walks up from sourceDir to find the project root
// (directory containing pyproject.toml or setup.py).
func findProjectRoot(sourceDir string) string {
	dir := sourceDir
	for i := 0; i < 10; i++ {
		for _, marker := range []string{"pyproject.toml", "setup.py", "setup.cfg"} {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return sourceDir
}

// runPythonExtract writes the extraction script to a temp file
// and runs it with the given Python interpreter.
func runPythonExtract(pythonPath, sourceDir, appSpec string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "pacto-extract-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	scriptPath := filepath.Join(tmpDir, "extract_fastapi.py")
	if err := os.WriteFile(scriptPath, []byte(extractFastAPIScript), 0o644); err != nil {
		return "", fmt.Errorf("failed to write script: %w", err)
	}

	projectRoot := findProjectRoot(sourceDir)

	var args []string
	isUv := filepath.Base(pythonPath) == "uv"

	if isUv {
		// Use "uv run python" to leverage the project's virtualenv
		args = []string{"run", "--project", projectRoot, "python", scriptPath, sourceDir}
	} else {
		args = []string{scriptPath, sourceDir}
	}

	if appSpec != "" {
		args = append(args, appSpec)
	}

	cmd := exec.Command(pythonPath, args...)
	cmd.Dir = projectRoot

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := stderr.String()
		if stderrStr != "" {
			return "", fmt.Errorf("python extraction failed: %w\nstderr: %s", err, stderrStr)
		}
		return "", fmt.Errorf("python extraction failed: %w", err)
	}

	raw := stdout.String()

	// Extract JSON between markers (app may emit logging to stdout).
	const startMarker = "__PACTO_OPENAPI_START__"
	const endMarker = "__PACTO_OPENAPI_END__"
	startIdx := strings.Index(raw, startMarker)
	endIdx := strings.Index(raw, endMarker)
	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		return "", fmt.Errorf("python extraction: markers not found in output")
	}
	output := strings.TrimSpace(raw[startIdx+len(startMarker) : endIdx])
	if output == "" {
		return "", fmt.Errorf("python extraction returned empty output")
	}

	return output, nil
}
