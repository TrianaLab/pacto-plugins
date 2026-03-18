# pacto-plugin-schema-infer

A [Pacto](https://github.com/trianalab/pacto) plugin that reads a service
configuration file (JSON, YAML, or TOML) and infers a
[JSON Schema](https://json-schema.org/) from it.

## Installation

```bash
make install
```

Or download a pre-built binary from the
[latest release](https://github.com/trianalab/pacto-plugins/releases/latest).

## Usage

```bash
pacto generate schema-infer <bundle-dir> --option file=config.yaml
```

### Arguments

| Argument | Description |
|----------|-------------|
| `<bundle-dir>` | Path to the Pacto bundle directory containing the service's source code and `pacto.yaml`. Use `.` for the current directory. |

### Example

```bash
# Run from the project root
pacto generate schema-infer .  --option file=config.yaml

# Or specify an explicit path
pacto generate schema-infer /path/to/my-service --option file=config.yaml
```

This generates a `config.schema.json` file containing the inferred JSON Schema.
Reference it in your Pacto contract:

```yaml
configuration:
  schema: config.schema.json
```

### Version

```bash
pacto-plugin-schema-infer --version
```

## Options

Options are passed via `--option key=value` flags when invoking with Pacto.

### `file` (required)

Path to the configuration file to infer from, relative to the bundle directory.

```bash
pacto generate schema-infer . --option file=config.yaml
```

Supported file extensions: `.json`, `.yaml`, `.yml`, `.toml`. The extension
determines which parser is used. If the extension is not recognized, the
plugin exits with an error.

### `title` (optional)

Title for the generated JSON Schema's `title` field.

- **Default**: `"configuration"`

```bash
pacto generate schema-infer . --option file=config.yaml --option title="My Service Config"
```

The output schema always has a fixed output path of `config.schema.json`.

## Supported formats

| Format | Extensions     | Parser |
|--------|---------------|--------|
| JSON   | `.json`       | `encoding/json` |
| YAML   | `.yaml`, `.yml` | `gopkg.in/yaml.v3` |
| TOML   | `.toml`       | `github.com/BurntSushi/toml` |

## How it works

The plugin parses the configuration file and recursively infers JSON Schema
types from the values it finds:

| Go type          | JSON Schema type |
|------------------|------------------|
| `string`         | `string`         |
| `float64`        | `number`         |
| `int64` / `int`  | `integer`        |
| `bool`           | `boolean`        |
| `nil`            | `null`           |
| `map[string]any` | `object`         |
| `[]any`          | `array`          |

All keys found in the sample are marked as `required`. The root object sets
`additionalProperties: false`.

> **Note:** The generated schema infers types from sample values and marks all
> present fields as required. Review the output and relax constraints as needed
> for your use case.

## Example

Given `config.yaml`:

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  tls: true
logging:
  level: info
  outputs:
    - stdout
    - file
```

The plugin generates:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "title": "configuration",
  "properties": {
    "server": {
      "type": "object",
      "properties": {
        "host": { "type": "string" },
        "port": { "type": "number" },
        "tls": { "type": "boolean" }
      },
      "required": ["host", "port", "tls"]
    },
    "logging": {
      "type": "object",
      "properties": {
        "level": { "type": "string" },
        "outputs": {
          "type": "array",
          "items": { "type": "string" }
        }
      },
      "required": ["level", "outputs"]
    }
  },
  "required": ["logging", "server"]
}
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
