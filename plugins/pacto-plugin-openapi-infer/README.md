# pacto-plugin-openapi-infer

A [Pacto](https://github.com/trianalab/pacto) plugin that auto-detects web
frameworks and extracts OpenAPI 3.1 specs from source code.

## Installation

```bash
make install
```

Or download a pre-built binary from the
[latest release](https://github.com/trianalab/pacto-plugins/releases/latest).

## Usage

```bash
pacto generate openapi-infer <bundle-dir> [--option key=value ...]
```

### Arguments

| Argument | Description |
|----------|-------------|
| `<bundle-dir>` | Path to the Pacto bundle directory containing the service's source code and `pacto.yaml`. Use `.` for the current directory. |

### Examples

```bash
# Auto-detect framework and extract (default output: interfaces/openapi.yaml)
pacto generate openapi-infer .

# Or specify an explicit path
pacto generate openapi-infer /path/to/my-service

# Custom output path (format inferred from extension)
pacto generate openapi-infer . --option output=interfaces/openapi.json

# Override framework detection
pacto generate openapi-infer . --option framework=fastapi
```

This generates an OpenAPI spec file. Reference it in your Pacto contract:

```yaml
interfaces:
  - name: rest-api
    type: http
    port: 8080
    visibility: public
    contract: interfaces/openapi.yaml
```

### Version

```bash
pacto-plugin-openapi-infer --version
```

## Options

Options are passed via `--option key=value` flags when invoking with Pacto.

### `output` (optional)

Output file path for the generated OpenAPI spec.

- **Default**: `interfaces/openapi.yaml`
- **Format detection**: the file extension determines the output format:
  - `.json` -- outputs indented JSON
  - `.yaml` or `.yml` -- outputs YAML (default)

```bash
# YAML output (default)
pacto generate openapi-infer .

# JSON output
pacto generate openapi-infer . --option output=interfaces/openapi.json

# Custom path with YAML
pacto generate openapi-infer . --option output=api/spec.yaml
```

### `framework` (optional)

Override automatic framework detection. Useful when the project has
dependencies for multiple frameworks, or when the detection heuristic
doesn't match your setup.

- **Default**: auto-detected from dependency files
- **Possible values**:
  - `fastapi` -- force FastAPI runtime extraction. Requires the project's
    Python environment to be set up with FastAPI and all dependencies
    installed. Detects by checking `requirements.txt`, `pyproject.toml`,
    `Pipfile`, `setup.py`, or `setup.cfg` for the string `fastapi`.
  - `huma` -- force Huma static extraction via Go AST analysis. No runtime
    or build required. Detects by checking `go.mod` for
    `danielgtaylor/huma`.

```bash
# Skip detection, force FastAPI extraction
pacto generate openapi-infer . --option framework=fastapi

# Skip detection, force Huma extraction
pacto generate openapi-infer . --option framework=huma
```

If framework is not specified and no supported framework is detected, the
plugin exits with an error listing the supported frameworks.

## Supported frameworks

| Framework | Language | Detection | Extraction |
|-----------|----------|-----------|------------|
| [FastAPI](https://fastapi.tiangolo.com/) | Python | `fastapi` in dependency files | Runtime -- runs `app.openapi()` |
| [Huma](https://huma.rocks/) | Go | `danielgtaylor/huma` in `go.mod` | Static -- Go AST analysis |

### FastAPI (runtime extraction)

Extracts the exact OpenAPI spec by running the FastAPI app using the project's
Python environment. The plugin auto-discovers:

- **Python interpreter**: walks up from the project directory looking for
  `.venv/bin/python`, `venv/bin/python`, or uses `uv run` if a `pyproject.toml`
  is present, falling back to system `python3`/`python`
- **FastAPI app**: scans `*.py` files for `FastAPI()` instances and imports
  the app object to call `app.openapi()`

**Requirements**: the project's Python environment must be set up with all
dependencies installed.

The `/health` endpoint is automatically excluded from the generated spec.

### Huma (static extraction)

Extracts routes and schemas using Go's built-in `go/ast` parser. Supports:

- `huma.Get`, `huma.Post`, `huma.Put`, `huma.Delete`, `huma.Patch` shorthand calls
- `huma.Register` with `huma.Operation` struct (extracts method, path, summary)
- `huma.DefaultConfig` for API title and version
- Exported Go structs with JSON tags as schema definitions
- Path parameters extracted from route patterns (e.g., `/pets/{id}`)

No build, runtime, or external dependencies required.

## How it works

1. The plugin receives the project's `bundleDir` path
2. It detects the web framework by checking dependency files
3. It extracts the OpenAPI spec using the framework-specific strategy
4. The spec is formatted as YAML or JSON based on the output file extension
5. The result is returned as a Pacto `GenerateResponse`

## Example

Given a Huma project with:

```go
func main() {
    cfg := huma.DefaultConfig("Pet Store", "1.0.0")
    // ...
    huma.Get(api, "/pets", listPets)
    huma.Get(api, "/pets/{petId}", getPet)
    huma.Post(api, "/pets", createPet)
}
```

Running:

```bash
pacto generate openapi-infer .
```

Generates `interfaces/openapi.yaml`:

```yaml
openapi: 3.1.0
info:
    title: Pet Store
    version: 1.0.0
paths:
    /pets:
        get:
            responses:
                "200":
                    description: Successful response
        post:
            responses:
                "200":
                    description: Successful response
    /pets/{petId}:
        get:
            parameters:
                - in: path
                  name: petId
                  required: true
                  schema:
                    type: string
            responses:
                "200":
                    description: Successful response
```

## Development

```bash
make build      # Build the binary to dist/
make test       # Run unit tests
make e2e        # Run e2e tests
make coverage   # Run tests with coverage report
make lint       # Check formatting and run go vet
make ci         # Run full CI pipeline
make install    # Install to $GOPATH/bin
```
