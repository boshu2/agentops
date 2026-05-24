#!/usr/bin/env bash
# Validate manifest files against versioned schemas.
# Usage: ./scripts/validate-manifests.sh [--repo-root <path>]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

usage() {
    cat <<'EOF'
Usage: ./scripts/validate-manifests.sh [--repo-root <path>]

Options:
  --repo-root <path>  Validate manifests under a specific repo root.
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
SKILL_FRONTMATTER_HELPER=""
SKILL_FRONTMATTER_HELPER_DIR=""

cleanup() {
    if [[ -n "$SKILL_FRONTMATTER_HELPER_DIR" && -d "$SKILL_FRONTMATTER_HELPER_DIR" ]]; then
        rm -rf "$SKILL_FRONTMATTER_HELPER_DIR"
    fi
}

trap cleanup EXIT

if ! command -v jq >/dev/null 2>&1; then
    echo "error: jq is required" >&2
    exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
    echo "error: python3 is required" >&2
    exit 1
fi

build_skill_frontmatter_helper() {
    local bin_ext

    if [[ -n "$SKILL_FRONTMATTER_HELPER" && -x "$SKILL_FRONTMATTER_HELPER" ]]; then
        return 0
    fi

    if ! command -v go >/dev/null 2>&1; then
        echo "error: go is required to validate skill frontmatter when python3 lacks PyYAML" >&2
        return 1
    fi

    if [[ ! -d "$REPO_ROOT/cli" ]]; then
        echo "error: cli/ module not found under $REPO_ROOT" >&2
        return 1
    fi

    SKILL_FRONTMATTER_HELPER_DIR="$(mktemp -d)"
    bin_ext=""
    case "$(uname -s)" in
        MINGW*|MSYS*|CYGWIN*)
            bin_ext=".exe"
            ;;
    esac
    SKILL_FRONTMATTER_HELPER="$SKILL_FRONTMATTER_HELPER_DIR/skill-frontmatter-json$bin_ext"

    if ! (cd "$REPO_ROOT/cli" && go build -o "$SKILL_FRONTMATTER_HELPER" ./cmd/skill-frontmatter-json); then
        rm -rf "$SKILL_FRONTMATTER_HELPER_DIR"
        SKILL_FRONTMATTER_HELPER_DIR=""
        SKILL_FRONTMATTER_HELPER=""
        return 1
    fi

    return 0
}

extract_skill_frontmatter_json() {
    local skill_md="$1"
    local output
    local status

    set +e
    output="$(
        python3 - "$skill_md" <<'PY'
import json
import re
import sys

with open(sys.argv[1], "r", encoding="utf-8") as handle:
    content = handle.read()

m = re.match(r'^\ufeff?---\s*\r?\n(.*?)\r?\n---', content, re.DOTALL)
if not m:
    print("{}")
    sys.exit(0)

try:
    import yaml
except ModuleNotFoundError:
    sys.exit(3)

data = yaml.safe_load(m.group(1))
if not isinstance(data, dict):
    print("{}")
    sys.exit(0)

print(json.dumps(data))
PY
    )"
    status=$?
    set -e

    if [[ "$status" -eq 0 ]]; then
        printf '%s\n' "$output"
        return 0
    fi

    if [[ "$status" -ne 3 ]]; then
        return "$status"
    fi

    build_skill_frontmatter_helper || return 1
    "$SKILL_FRONTMATTER_HELPER" "$skill_md"
}

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

validate_manifest \
    "$REPO_ROOT/.codex-plugin/plugin.json" \
    "$REPO_ROOT/schemas/codex-plugin-manifest.v1.schema.json" \
    "Codex plugin manifest"

validate_manifest \
    "$REPO_ROOT/plugins/marketplace.json" \
    "$REPO_ROOT/schemas/codex-marketplace.v1.schema.json" \
    "Codex marketplace manifest"

# --- Skill Frontmatter Validation ---
log "Validating skill frontmatter"
SKILL_SCHEMA="$REPO_ROOT/schemas/skill-frontmatter.v1.schema.json"
if [[ -f "$SKILL_SCHEMA" ]]; then
    for skill_md in "$REPO_ROOT"/skills/*/SKILL.md; do
        [[ -f "$skill_md" ]] || continue
        skill_name="$(basename "$(dirname "$skill_md")")"

        # Extract YAML frontmatter and convert to JSON.
        # Prefer PyYAML when present, but fall back to a repo-local Go helper so
        # the gate is not coupled to a globally provisioned Python environment.
        frontmatter_json="$(extract_skill_frontmatter_json "$skill_md")" || continue

        # Skip if empty/no frontmatter
        if [[ "$frontmatter_json" == "{}" ]]; then
            continue
        fi

        # Write to temp file and validate via existing function
        tmp_manifest="$(mktemp)"
        echo "$frontmatter_json" > "$tmp_manifest"
        validate_manifest "$tmp_manifest" "$SKILL_SCHEMA" "skill/$skill_name frontmatter"
        rm -f "$tmp_manifest"
    done
fi

# --- Memory Packet Validation ---
log "Validating memory packets"
MEMORY_SCHEMA="$REPO_ROOT/schemas/memory-packet.v1.schema.json"
if [[ -f "$MEMORY_SCHEMA" ]]; then
    found_memory=0
    for packet in "$REPO_ROOT"/.agents/memory/*.json; do
        [[ -f "$packet" ]] || continue
        found_memory=1
        validate_manifest "$packet" "$MEMORY_SCHEMA" "memory/$(basename "$packet")"
    done
    if [[ "$found_memory" -eq 0 ]]; then
        echo "ℹ no memory packets found (skipped)"
    fi
fi

# --- Handoff Artifact Validation ---
log "Validating handoff artifacts"
HANDOFF_SCHEMA="$REPO_ROOT/schemas/handoff.v1.schema.json"
if [[ -f "$HANDOFF_SCHEMA" ]]; then
    found_handoff=0
    for handoff in "$REPO_ROOT"/.agents/handoff/*.json; do
        [[ -f "$handoff" ]] || continue
        found_handoff=1
        validate_manifest "$handoff" "$HANDOFF_SCHEMA" "handoff/$(basename "$handoff")"
    done
    if [[ "$found_handoff" -eq 0 ]]; then
        echo "ℹ no handoff artifacts found (skipped)"
    fi
fi

# --- Evidence-Only Closure Artifact Validation ---
log "Validating evidence-only closure artifacts"
EVIDENCE_ONLY_CLOSURE_SCHEMA="$REPO_ROOT/schemas/evidence-only-closure.v1.schema.json"
if [[ -f "$EVIDENCE_ONLY_CLOSURE_SCHEMA" ]]; then
    found_evidence_only_closure=0
    for artifact_dir in \
        "$REPO_ROOT"/.agents/council/evidence-only-closures \
        "$REPO_ROOT"/.agents/releases/evidence-only-closures; do
        [[ -d "$artifact_dir" ]] || continue
        for artifact in "$artifact_dir"/*.json; do
            [[ -f "$artifact" ]] || continue
            found_evidence_only_closure=1
            validate_manifest "$artifact" "$EVIDENCE_ONLY_CLOSURE_SCHEMA" "evidence-only-closure/$(basename "$artifact")"
        done
    done
    if [[ "$found_evidence_only_closure" -eq 0 ]]; then
        echo "ℹ no evidence-only closure artifacts found (skipped)"
    fi
fi

if [[ "$errors" -gt 0 ]]; then
    exit 1
fi

exit 0
