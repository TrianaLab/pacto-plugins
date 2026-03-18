# Contributing to pacto-plugins

Thank you for your interest in contributing to pacto-plugins! This guide will help you get started.

## Code of Conduct

By participating in this project, you agree to treat all contributors with respect and maintain a welcoming, inclusive environment.

## Getting Started

### Prerequisites

- [Go 1.25+](https://go.dev/dl/)
- [Git](https://git-scm.com/)
- A terminal with `make` available

### Setting Up Your Development Environment

1. **Fork and clone the repository:**

   ```bash
   git clone https://github.com/<your-username>/pacto-plugins.git
   cd pacto-plugins
   ```

2. **Install dependencies for all plugins:**

   ```bash
   for plugin in plugins/*/; do (cd "$plugin" && go mod download); done
   ```

3. **Build all plugins:**

   ```bash
   make build
   ```

4. **Run the tests:**

   ```bash
   make test    # unit tests
   make e2e     # end-to-end tests
   make lint    # linter
   make ci      # all of the above
   ```

## How to Contribute

### Reporting Bugs

If you find a bug, please [open an issue](https://github.com/TrianaLab/pacto-plugins/issues/new). Include:

- Steps to reproduce the issue
- Expected vs. actual behavior
- Your environment (OS, Go version, plugin version)
- Relevant logs or error messages

### Suggesting Features

Have an idea? [Open a feature request](https://github.com/TrianaLab/pacto-plugins/issues/new). Describe the problem you're trying to solve and the solution you'd like to see.

### Submitting Changes

1. **Create a branch** from `main`:

   ```bash
   git checkout -b feat/my-feature
   ```

   Use a descriptive branch name with a prefix: `feat/`, `fix/`, `docs/`, `refactor/`, `test/`.

2. **Make your changes.** Keep commits focused and atomic.

3. **Write or update tests.** All new functionality should include tests. All bug fixes should include a regression test.

4. **Run the full check suite before pushing:**

   ```bash
   make ci
   ```

5. **Write a clear commit message** following the project's convention:

   ```
   feat: add support for Spring Boot framework detection
   fix: resolve nested schema parsing in huma extractor
   docs: update openapi-infer README with new options
   ```

   Use the format `<type>: <description>` where type is one of: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `ci`.

6. **Open a pull request** against `main`. Fill in the PR template and link any related issues.

## Development Guidelines

### Repository Structure

```
pacto-plugins/
  plugins/
    pacto-plugin-schema-infer/     # Each plugin is a standalone Go module
      cmd/main.go                  # Plugin entrypoint
      internal/                    # Internal packages
      tests/e2e/                   # End-to-end tests
      Makefile                     # Per-plugin build targets
    pacto-plugin-openapi-infer/
      ...
  .github/
    actions/ci/                    # Shared CI action
    workflows/                     # CI, PR, and release workflows
  Makefile                         # Root Makefile orchestrates all plugins
```

### Code Style

- Follow standard Go conventions and idioms.
- Code must pass `gofmt`, `go vet`, `gocyclo` (threshold: 15), and `golangci-lint`.
- Keep functions small and focused. Avoid deep nesting.
- Use meaningful names for variables, functions, and packages.

### Testing

- **Unit tests** live alongside the code they test (`_test.go` files).
- **End-to-end tests** live in `tests/e2e/` and use the `e2e` build tag.
- Aim for meaningful test coverage. Cover edge cases and error paths, not just the happy path.
- Run `make coverage` to generate a coverage report for each plugin.

### Adding a New Plugin

1. Create a new directory under `plugins/` with its own `go.mod`.
2. Implement the [Pacto plugin protocol](https://trianalab.github.io/pacto/plugins/) in `cmd/main.go`.
3. Add unit tests targeting 100% coverage on `internal/` packages.
4. Add e2e tests in `tests/e2e/` with the `e2e` build tag.
5. Add a `Makefile` following the existing plugins' structure.
6. Add a `README.md` documenting usage, options, and examples.
7. CI will automatically discover and test the new plugin.

## Pull Request Process

1. Ensure CI passes (lint, unit tests, e2e tests).
2. Request a review from a maintainer.
3. Address review feedback. Push new commits rather than force-pushing so reviewers can see incremental changes.
4. Once approved, a maintainer will merge your PR.

## Releasing

Releases are managed by maintainers. The release workflow is triggered by publishing a GitHub Release. Binaries for all plugins are cross-compiled and attached to the release.

## Questions?

If you're unsure about anything, feel free to [open a discussion](https://github.com/TrianaLab/pacto-plugins/issues) or ask in your pull request. We're happy to help!

## License

By contributing to pacto-plugins, you agree that your contributions will be licensed under the [MIT License](LICENSE).
