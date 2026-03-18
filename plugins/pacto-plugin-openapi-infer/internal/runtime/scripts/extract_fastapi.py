"""Extract OpenAPI spec from a FastAPI application.

Imports the FastAPI app instance, calls app.openapi(), and prints
the JSON spec to stdout.

Usage: python extract_fastapi.py <source_dir> [<app_module>:<app_var>]

This script should be run using the project's own Python environment
(virtualenv/venv) which has all dependencies installed.
"""

import importlib
import json
import re
import sys
from pathlib import Path


def find_app_candidates(source_dir):
    """Scan Python files for FastAPI app instances.

    Returns list of (module_path, var_name) tuples.
    """
    candidates = []
    source_path = Path(source_dir)
    pattern = re.compile(r"(\w+)\s*=\s*FastAPI\(")

    for py_file in source_path.rglob("*.py"):
        try:
            content = py_file.read_text()
        except Exception:
            continue

        if "FastAPI(" not in content:
            continue

        for match in pattern.finditer(content):
            var_name = match.group(1)
            rel = py_file.relative_to(source_path)
            module_parts = list(rel.parts[:-1]) + [rel.stem]
            module_path = ".".join(module_parts)
            candidates.append((module_path, var_name))

    return candidates


def extract_openapi(source_dir, app_spec=None):
    """Import the FastAPI app and extract its OpenAPI spec."""
    # Add source dir to Python path
    sys.path.insert(0, source_dir)

    # Also add parent dir (for package imports)
    parent = str(Path(source_dir).parent)
    if parent not in sys.path:
        sys.path.insert(0, parent)

    if app_spec:
        module_path, var_name = app_spec.rsplit(":", 1)
        mod = importlib.import_module(module_path)
        app = getattr(mod, var_name)
    else:
        candidates = find_app_candidates(source_dir)
        if not candidates:
            print("ERROR: No FastAPI app found in source directory",
                  file=sys.stderr)
            sys.exit(1)

        # Prefer 'app' variable name
        candidates.sort(key=lambda c: (c[1] != "app", c[0]))

        app = None
        last_err = None
        for module_path, var_name in candidates:
            try:
                mod = importlib.import_module(module_path)
                app = getattr(mod, var_name)
                break
            except Exception as e:
                last_err = e
                print(
                    "WARN: Failed to import "
                    f"{module_path}:{var_name}: {e}",
                    file=sys.stderr,
                )
                continue

        if app is None:
            print(f"ERROR: Could not import any FastAPI app candidate: {last_err}",
                  file=sys.stderr)
            sys.exit(1)

    spec = app.openapi()

    # Remove /health endpoint (common pattern, excluded from contracts)
    if "paths" in spec and "/health" in spec["paths"]:
        del spec["paths"]["/health"]

    return spec


def main():
    if len(sys.argv) < 2:
        print("Usage: extract_fastapi.py <source_dir> [<module>:<var>]",
              file=sys.stderr)
        sys.exit(1)

    source_dir = sys.argv[1]
    app_spec = sys.argv[2] if len(sys.argv) > 2 else None

    spec = extract_openapi(source_dir, app_spec)
    # Use markers so the Go side can extract the JSON even if other
    # output (logging, warnings) is mixed into stdout.
    print("__PACTO_OPENAPI_START__")
    print(json.dumps(spec, indent=2))
    print("__PACTO_OPENAPI_END__")


if __name__ == "__main__":
    main()
