#!/usr/bin/env bash
# Validate manifest files against versioned schemas.
# Usage: ./scripts/validate-manifests.sh [--repo-root <path>] [--skip-hooks]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SKIP_HOOKS=0

usage() {
    cat <<'EOF'
Usage: ./scripts/validate-manifests.sh [--repo-root <path>] [--skip-hooks]

Options:
  --repo-root <path>  Validate manifests under a specific repo root.
  --skip-hooks        Skip hooks/hooks.json validation.
  -h, --help          Show this help message.
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --repo-root)
            if [[ $# -lt 2 ]]; then
                echo "error: --repo-root requires a value" >&2
                usage
                exit 2
            fi
            REPO_ROOT="$2"
            shift 2
            ;;
        --skip-hooks)
            SKIP_HOOKS=1
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "error: unknown argument: $1" >&2
            usage
            exit 2
            ;;
    esac
done

REPO_ROOT="$(cd "$REPO_ROOT" && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

pass() { echo -e "${GREEN}✓${NC} $1"; }
fail() { echo -e "${RED}✗${NC} $1"; errors=$((errors + 1)); }
log() { echo -e "${BLUE}==>${NC} $1"; }

errors=0

if ! command -v jq >/dev/null 2>&1; then
    echo "error: jq is required" >&2
    exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
    echo "error: python3 is required" >&2
    exit 1
fi

validate_manifest() {
    local manifest="$1"
    local schema="$2"
    local label="$3"
    local manifest_dir
    local declared_schema
    local output

    if [[ ! -f "$manifest" ]]; then
        fail "$label missing manifest: $manifest"
        return
    fi

    if [[ ! -f "$schema" ]]; then
        fail "$label missing schema: $schema"
        return
    fi

    if ! jq empty "$schema" >/dev/null 2>&1; then
        fail "$label schema is not valid JSON: $schema"
        return
    fi

    if ! jq empty "$manifest" >/dev/null 2>&1; then
        fail "$label manifest is not valid JSON: $manifest"
        return
    fi

    declared_schema="$(jq -r '."$schema" // empty' "$manifest")"
    if [[ -n "$declared_schema" ]]; then
        manifest_dir="$(cd "$(dirname "$manifest")" && pwd)"
        if ! output="$(
            python3 - "$manifest_dir" "$declared_schema" "$schema" <<'PY'
import os
import sys

manifest_dir, declared_schema, expected_schema = sys.argv[1:4]
resolved = os.path.abspath(os.path.normpath(os.path.join(manifest_dir, declared_schema)))
expected = os.path.abspath(expected_schema)
if resolved != expected:
    print(f"schema pointer resolves to {resolved}, expected {expected}")
    sys.exit(1)
PY
        )"; then
            fail "$label schema pointer drift detected"
            if [[ -n "$output" ]]; then
                while IFS= read -r line; do
                    echo "    $line"
                done <<<"$output"
            fi
            return
        fi
    else
        echo "ℹ $label manifest missing \$schema pointer (allowed)"
    fi

    if ! output="$(
        python3 - "$schema" "$manifest" <<'PY'
import json
import re
import sys

schema_path, data_path = sys.argv[1:3]

with open(schema_path, "r", encoding="utf-8") as handle:
    root_schema = json.load(handle)

with open(data_path, "r", encoding="utf-8") as handle:
    document = json.load(handle)

errors = []


def json_type_name(value):
    if value is None:
        return "null"
    if isinstance(value, bool):
        return "boolean"
    if isinstance(value, int):
        return "integer"
    if isinstance(value, float):
        return "number"
    if isinstance(value, str):
        return "string"
    if isinstance(value, list):
        return "array"
    if isinstance(value, dict):
        return "object"
    return type(value).__name__


def matches_type(expected, value):
    if expected == "null":
        return value is None
    if expected == "boolean":
        return isinstance(value, bool)
    if expected == "integer":
        return isinstance(value, int) and not isinstance(value, bool)
    if expected == "number":
        return (isinstance(value, int) or isinstance(value, float)) and not isinstance(value, bool)
    if expected == "string":
        return isinstance(value, str)
    if expected == "array":
        return isinstance(value, list)
    if expected == "object":
        return isinstance(value, dict)
    return True


def resolve_ref(ref):
    if not ref.startswith("#/"):
        raise ValueError(f"unsupported $ref: {ref}")
    node = root_schema
    for part in ref[2:].split("/"):
        part = part.replace("~1", "/").replace("~0", "~")
        if isinstance(node, dict) and part in node:
            node = node[part]
        else:
            raise ValueError(f"unresolvable $ref: {ref}")
    return node


def validate(schema, value, path):
    if "$ref" in schema:
        try:
            target = resolve_ref(schema["$ref"])
        except ValueError as error:
            errors.append(f"{path}: {error}")
            return
        validate(target, value, path)
        return

    expected_type = schema.get("type")
    if expected_type is not None:
        if isinstance(expected_type, list):
            if not any(matches_type(item, value) for item in expected_type):
                errors.append(f"{path}: expected one of {expected_type}, got {json_type_name(value)}")
                return
        elif not matches_type(expected_type, value):
            errors.append(f"{path}: expected {expected_type}, got {json_type_name(value)}")
            return

    if "const" in schema and value != schema["const"]:
        errors.append(f"{path}: expected const {schema['const']!r}, got {value!r}")

    if "enum" in schema and value not in schema["enum"]:
        errors.append(f"{path}: value {value!r} not in enum {schema['enum']!r}")

    if isinstance(value, str) and "minLength" in schema and len(value) < schema["minLength"]:
        errors.append(f"{path}: string shorter than minLength {schema['minLength']}")

    if isinstance(value, list):
        if "minItems" in schema and len(value) < schema["minItems"]:
            errors.append(f"{path}: expected at least {schema['minItems']} items")
        if "items" in schema:
            item_schema = schema["items"]
            for index, item in enumerate(value):
                validate(item_schema, item, f"{path}[{index}]")

    if isinstance(value, dict):
        required = schema.get("required", [])
        for key in required:
            if key not in value:
                errors.append(f"{path}: missing required property '{key}'")

        properties = schema.get("properties", {})
        additional = schema.get("additionalProperties", True)
        for key, item in value.items():
            item_path = f"{path}.{key}" if path != "$" else f"$.{key}"
            if key in properties:
                validate(properties[key], item, item_path)
            elif additional is False:
                errors.append(f"{path}: additional property '{key}' not allowed")
            elif isinstance(additional, dict):
                validate(additional, item, item_path)


validate(root_schema, document, "$")

if errors:
    for line in errors:
        print(line)
    sys.exit(1)
PY
    )"; then
        fail "$label failed schema validation"
        if [[ -n "$output" ]]; then
            while IFS= read -r line; do
                echo "    $line"
            done <<<"$output"
        fi
        return
    fi

    pass "$label matches $(basename "$schema")"
}

log "Validating manifest schemas"

validate_manifest \
    "$REPO_ROOT/.claude-plugin/plugin.json" \
    "$REPO_ROOT/schemas/plugin-manifest.v1.schema.json" \
    "plugin manifest"

if [[ "$SKIP_HOOKS" -eq 0 ]]; then
    validate_manifest \
        "$REPO_ROOT/hooks/hooks.json" \
        "$REPO_ROOT/schemas/hooks-manifest.v1.schema.json" \
        "hooks manifest"
fi

if [[ "$errors" -gt 0 ]]; then
    exit 1
fi

exit 0
