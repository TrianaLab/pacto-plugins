# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |

Only the latest release is actively supported with security updates. We recommend always running the most recent version.

## Reporting a Vulnerability

If you discover a security vulnerability in pacto-plugins, please report it responsibly. **Do not open a public GitHub issue.**

### How to Report

1. **Email:** Send a detailed report to the maintainers via [GitHub Security Advisories](https://github.com/TrianaLab/pacto-plugins/security/advisories/new).
2. Include the following in your report:
   - A description of the vulnerability
   - Steps to reproduce the issue
   - The potential impact
   - Any suggested fixes (if applicable)

### What to Expect

- **Acknowledgment:** We will acknowledge receipt of your report within **48 hours**.
- **Updates:** We will provide status updates as we investigate and work on a fix.
- **Disclosure:** Once a fix is released, we will coordinate with you on public disclosure. We aim to resolve critical issues within **30 days**.

## Security Practices

- Plugins run at **build time and CI time only** -- they have no runtime agents, sidecars, or persistent infrastructure.
- Each plugin is a standalone binary that reads JSON from stdin and writes JSON to stdout.
- All dependencies are kept up to date and monitored for known vulnerabilities.

## Scope

The following are in scope for security reports:

- Official plugins in this repository (`pacto-plugin-schema-infer`, `pacto-plugin-openapi-infer`)
- The plugin protocol implementation (stdin/stdout JSON handling)
- Runtime extraction scripts (e.g., FastAPI Python extraction)

The following are **out of scope**:

- The [Pacto CLI](https://github.com/TrianaLab/pacto) itself (report to that repository)
- Third-party integrations or tools consuming plugin output
- Vulnerabilities in upstream dependencies (report these to the upstream project, but let us know so we can update)
