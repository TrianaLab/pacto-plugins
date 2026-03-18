# pacto-plugins

Official plugins for [Pacto](https://github.com/trianalab/pacto) --
the OCI-distributed contract standard for cloud-native services.

## Available plugins

| Plugin | Description |
|--------|-------------|
| [pacto-plugin-schema-infer](./plugins/pacto-plugin-schema-infer) | Infer a JSON Schema from a sample config file (JSON, YAML, TOML) |
| [pacto-plugin-openapi-infer](./plugins/pacto-plugin-openapi-infer) | Auto-detect and extract OpenAPI 3.1 specs from source code (FastAPI, Huma) |

## Installation

### Download a release binary

Download the binary for your platform from the
[latest release](https://github.com/trianalab/pacto-plugins/releases/latest)
and place it in your `$PATH` (or `$(go env GOPATH)/bin`).

### Build from source

Install all plugins:

```bash
make install
```

Install a single plugin:

```bash
cd plugins/pacto-plugin-schema-infer
make install
```

## How plugins work

Each plugin is a standalone binary that implements the
[Pacto plugin protocol](https://trianalab.github.io/pacto/plugins/).
Pacto invokes a plugin by running its binary, sending a JSON request on
stdin, and reading a JSON response from stdout.

**Request** (stdin):

```json
{
  "bundleDir": "/path/to/project",
  "options": {
    "key": "value"
  }
}
```

**Response** (stdout):

```json
{
  "files": [
    { "path": "relative/output.yaml", "content": "..." }
  ],
  "message": "Human-readable summary"
}
```

## Development

### Repository structure

```
pacto-plugins/
  plugins/
    pacto-plugin-schema-infer/    # Each plugin is a standalone Go module
    pacto-plugin-openapi-infer/
  .github/
    actions/ci/                   # Shared CI action
    workflows/                    # CI, PR, and release workflows
  Makefile                        # Root Makefile orchestrates all plugins
```

### Make targets

```bash
make build      # Build all plugins to dist/
make test       # Run unit tests for all plugins
make e2e        # Run e2e tests for all plugins
make coverage   # Run tests with coverage for all plugins
make lint       # Run linters (gofmt, go vet) for all plugins
make ci         # Run full CI pipeline (lint + test + e2e)
make install    # Install all plugins to $GOPATH/bin
make clean      # Remove build artifacts
```

### Adding a new plugin

1. Create a new directory under `plugins/` with a Go module
2. Implement the plugin protocol in `cmd/main.go`
3. Add unit tests (target: 100% coverage on `internal/` packages)
4. Add e2e tests in `tests/e2e/` (build tag: `e2e`)
5. Add a `Makefile` following the existing plugins' structure
6. Open a PR -- CI will automatically discover and test the new plugin

### CI

CI runs automatically on PRs and pushes to `main`. Each plugin is discovered
dynamically and tested in a matrix job. The pipeline includes:

- `gofmt` formatting check
- `go vet` static analysis
- `gocyclo` complexity check (threshold: 15)
- `golangci-lint`
- Unit tests
- E2E tests
- Coverage reporting to Codecov

### Releases

Releases are triggered by publishing a GitHub Release. The workflow
cross-compiles each plugin for 6 platform/arch combinations:

| OS | Architecture |
|----|-------------|
| Linux | amd64, arm64 |
| macOS | amd64, arm64 |
| Windows | amd64, arm64 |

Binaries are attached to the GitHub Release as downloadable assets.

## License

MIT
