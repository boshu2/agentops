#!/usr/bin/env bash
set -euo pipefail

# check-contract-compatibility.sh
# Dynamic contract-compatibility gate.
#
# Validates:
#   1. All contract files referenced in docs/INDEX.md exist on disk
#   2. All contract .md files' embedded schema/file references resolve
#   3. All *.schema.json files are valid JSON
#   4. Orphan allowlist is well-formed and governed
#   5. All contracts on disk are catalogued in docs/INDEX.md (orphan check)
#   6. Example JSON files conform to their corresponding schemas

# Graceful skip when required tools are missing
for tool in jq python3; do
  if ! command -v "$tool" &>/dev/null; then
    echo "WARN: $tool not found — skipping contract compatibility check"
    exit 0
  fi
done

ROOT="${1:-.}"
if [[ ! -d "$ROOT" ]]; then
  echo "FAIL: repository root not found: $ROOT"
  exit 1
fi
ROOT="$(cd "$ROOT" && pwd)"

CONTRACTS_DIR="$ROOT/docs/contracts"
INDEX="$ROOT/docs/INDEX.md"
BRIDGE="$ROOT/docs/ol-bridge-contracts.md"
ORPHAN_ALLOWLIST="$ROOT/scripts/contract-orphans-allowlist.txt"

failures=0
warnings=0
INDEX_CONTRACTS_TMP="$(mktemp)"
ALLOWLIST_ENTRIES_TMP="$(mktemp)"
ALLOWLIST_PATHS_TMP="$(mktemp)"

fail() { echo "FAIL: $1"; failures=$((failures + 1)); }
warn() { echo "WARN: $1"; warnings=$((warnings + 1)); }
pass() { echo "  OK: $1"; }
trim() {
  printf '%s' "$1" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//'
}

cleanup() {
  rm -f "$INDEX_CONTRACTS_TMP" "$ALLOWLIST_ENTRIES_TMP" "$ALLOWLIST_PATHS_TMP"
}
trap cleanup EXIT

load_orphan_allowlist() {
  local line_number=0
  local today
  today="$(date +%F)"

  if [[ ! -f "$ORPHAN_ALLOWLIST" ]]; then
    fail "orphan allowlist not found: scripts/contract-orphans-allowlist.txt"
    return
  fi

  while IFS= read -r raw_line || [[ -n "$raw_line" ]]; do
    line_number=$((line_number + 1))
    local stripped
    stripped="$(trim "$raw_line")"

    [[ -z "$stripped" ]] && continue
    [[ "$stripped" == \#* ]] && continue

    local path reason owner expires extra
    IFS='|' read -r path reason owner expires extra <<<"$stripped"
    path="$(trim "$path")"
    reason="$(trim "${reason:-}")"
    owner="$(trim "${owner:-}")"
    expires="$(trim "${expires:-}")"
    extra="$(trim "${extra:-}")"

    if [[ -n "$extra" || -z "$path" || -z "$reason" || -z "$owner" || -z "$expires" ]]; then
      fail "allowlist line $line_number malformed (expected: path | reason | owner | expires)"
      continue
    fi
    if [[ "$path" == *"*"* || "$path" == *"?"* || "$path" == *"["* || "$path" == *"]"* ]]; then
      fail "allowlist line $line_number path contains wildcard: $path"
      continue
    fi
    if [[ "$path" != docs/contracts/* ]] || [[ "$path" == "docs/contracts/" ]]; then
      fail "allowlist line $line_number path must be repo-relative under docs/contracts/: $path"
      continue
    fi
    if [[ "$path" == */../* ]] || [[ "$path" == ../* ]] || [[ "$path" == */.. ]]; then
      fail "allowlist line $line_number path traversal is not allowed: $path"
      continue
    fi
    if [[ ! "$expires" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]]; then
      fail "allowlist line $line_number expires must be YYYY-MM-DD: $expires"
      continue
    fi
    if [[ "$expires" < "$today" ]]; then
      fail "allowlist line $line_number entry expired on $expires: $path"
      continue
    fi
    if grep -Fxq "$path" "$ALLOWLIST_PATHS_TMP"; then
      fail "allowlist line $line_number duplicate path: $path"
      continue
    fi

    printf '%s|%s|%s|%s\n' "$path" "$reason" "$owner" "$expires" >>"$ALLOWLIST_ENTRIES_TMP"
    printf '%s\n' "$path" >>"$ALLOWLIST_PATHS_TMP"
  done < "$ORPHAN_ALLOWLIST"

  if [[ -s "$ALLOWLIST_ENTRIES_TMP" ]]; then
    local count
    count="$(wc -l < "$ALLOWLIST_ENTRIES_TMP" | tr -d ' ')"
    pass "Loaded $count orphan allowlist entrie(s)"
  else
    pass "No orphan allowlist entries configured"
  fi
}

echo "=== Contract compatibility gate ==="
echo ""

# ── Check 1: docs/contracts/ directory exists ──

if [[ ! -d "$CONTRACTS_DIR" ]]; then
  echo "WARN: docs/contracts/ directory not found — skipping contract compatibility check"
  exit 0
fi

# ── Check 2: Orphan allowlist format and policy ──

echo "--- Orphan allowlist validation ---"
load_orphan_allowlist
echo ""

# ── Check 3: INDEX.md references resolve ──

echo "--- INDEX.md link resolution ---"
if [[ -f "$INDEX" ]]; then
  # Extract markdown links pointing into contracts/ (outside code blocks)
  while IFS= read -r ref; do
    [[ -z "$ref" ]] && continue
    printf 'docs/%s\n' "$ref" >>"$INDEX_CONTRACTS_TMP"
    if [[ -f "$ROOT/docs/$ref" ]]; then
      pass "$ref"
    else
      fail "INDEX.md references $ref but file not found"
    fi
  done < <(awk '/^```/{skip=!skip; next} !skip{print}' "$INDEX" \
    | grep -oE '\]\(contracts/[A-Za-z0-9_./-]+\)' \
    | sed 's/\](//; s/)//' | sort -u)
else
  fail "docs/INDEX.md not found"
fi
sort -u "$INDEX_CONTRACTS_TMP" -o "$INDEX_CONTRACTS_TMP"
echo ""

# ── Check 4: Bridge doc references resolve ──

echo "--- Bridge doc reference resolution ---"
if [[ -f "$BRIDGE" ]]; then
  while IFS= read -r ref; do
    [[ -z "$ref" ]] && continue
    if [[ -f "$ROOT/$ref" ]]; then
      pass "$ref"
    else
      fail "ol-bridge-contracts.md references $ref but file not found"
    fi
  done < <(awk '/^```/{skip=!skip; next} !skip{print}' "$BRIDGE" \
    | grep -oE 'docs/contracts/[A-Za-z0-9_./-]+' | sort -u)
else
  warn "docs/ol-bridge-contracts.md not found (optional)"
fi
echo ""

# ── Check 5: Contract .md files' embedded references resolve ──

echo "--- Contract .md cross-references ---"
for md in "$CONTRACTS_DIR"/*.md; do
  [[ -f "$md" ]] || continue
  basename="$(basename "$md")"
  while IFS= read -r ref; do
    [[ -z "$ref" ]] && continue
    # Try resolving relative to contracts dir, then docs/, repo root, then schemas/
    if [[ -f "$CONTRACTS_DIR/$ref" ]] || [[ -f "$ROOT/docs/$ref" ]] || [[ -f "$ROOT/$ref" ]] || [[ -f "$ROOT/schemas/$ref" ]]; then
      pass "$basename -> $ref"
    else
      fail "$basename references $ref but file not found"
    fi
  done < <(awk '/^```/{skip=!skip; next} !skip{print}' "$md" \
    | grep -oE '[A-Za-z0-9_.-]+\.schema\.json' | sort -u)
done
echo ""

# ── Check 6: All *.schema.json files are valid JSON ──

echo "--- Schema JSON validation ---"
for schema in "$CONTRACTS_DIR"/*.schema.json; do
  [[ -f "$schema" ]] || continue
  basename="$(basename "$schema")"
  if jq empty "$schema" 2>/dev/null; then
    pass "$basename is valid JSON"
  else
    fail "$basename is not valid JSON"
  fi
done
# Also check schemas/ dir at repo root
if [[ -d "$ROOT/schemas" ]]; then
  for schema in "$ROOT/schemas"/*.schema.json; do
    [[ -f "$schema" ]] || continue
    basename="$(basename "$schema")"
    if jq empty "$schema" 2>/dev/null; then
      pass "schemas/$basename is valid JSON"
    else
      fail "schemas/$basename is not valid JSON"
    fi
  done
fi
echo ""

# ── Check 7: All *.json example files are valid JSON ──

echo "--- Example JSON validation ---"
for example in "$CONTRACTS_DIR"/*.example.json; do
  [[ -f "$example" ]] || continue
  basename="$(basename "$example")"
  if jq empty "$example" 2>/dev/null; then
    pass "$basename is valid JSON"
  else
    fail "$basename is not valid JSON"
  fi
done
echo ""

# ── Check 8: Example JSON files conform to their schemas ──

echo "--- Example-to-schema conformance ---"
for example in "$CONTRACTS_DIR"/*.example.json; do
  [[ -f "$example" ]] || continue
  base="$(basename "$example")"

  # Derive schema name: strip .example.json, find matching .schema.json
  # memrl-policy.profile.example.json -> memrl-policy.schema.json
  # Strategy: try progressively shorter prefixes
  schema_found=""
  prefix="${base%.example.json}"  # e.g. "memrl-policy.profile"
  while [[ "$prefix" == *.* ]]; do
    candidate="$CONTRACTS_DIR/${prefix%.*}.schema.json"
    if [[ -f "$candidate" ]]; then
      schema_found="$candidate"
      break
    fi
    prefix="${prefix%.*}"
  done
  # Also try direct: prefix.schema.json (no dots left to strip)
  if [[ -z "$schema_found" ]] && [[ -f "$CONTRACTS_DIR/${prefix}.schema.json" ]]; then
    schema_found="$CONTRACTS_DIR/${prefix}.schema.json"
  fi

  if [[ -z "$schema_found" ]]; then
    warn "no schema found for example: $base"
    continue
  fi

  schema_base="$(basename "$schema_found")"

  # Validate using embedded Python validator (same pattern as validate-manifests.sh)
  if ! output="$(
    python3 - "$schema_found" "$example" <<'PYVALIDATE'
import json
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
PYVALIDATE
  )"; then
    fail "$base failed schema validation against $schema_base"
    if [[ -n "$output" ]]; then
      while IFS= read -r line; do
        echo "    $line"
      done <<<"$output"
    fi
  else
    pass "$base conforms to $schema_base"
  fi
done
echo ""

# ── Check 9: Orphan detection — files on disk not in INDEX.md ──

echo "--- Orphan detection ---"
if [[ -f "$INDEX" ]]; then
  for contract in "$CONTRACTS_DIR"/*; do
    [[ -f "$contract" ]] || continue
    rel_contract="${contract#"$ROOT"/}"
    if grep -Fxq "$rel_contract" "$INDEX_CONTRACTS_TMP" 2>/dev/null; then
      pass "$rel_contract catalogued in INDEX.md"
    elif grep -Fxq "$rel_contract" "$ALLOWLIST_PATHS_TMP" 2>/dev/null; then
      metadata="$(grep -F "^$rel_contract|" "$ALLOWLIST_ENTRIES_TMP" | head -1 || true)"
      reason="$(trim "$(printf '%s' "$metadata" | cut -d'|' -f2)")"
      owner="$(trim "$(printf '%s' "$metadata" | cut -d'|' -f3)")"
      expires="$(trim "$(printf '%s' "$metadata" | cut -d'|' -f4)")"
      pass "$rel_contract allowlisted ($reason; $owner; expires $expires)"
    else
      fail "$rel_contract exists on disk but not in INDEX.md (not allowlisted)"
    fi
  done

  while IFS= read -r entry; do
    [[ -z "$entry" ]] && continue
    allowlisted_path="$(printf '%s' "$entry" | cut -d'|' -f1)"
    if [[ ! -f "$ROOT/$allowlisted_path" ]]; then
      fail "allowlist entry points to missing file: $allowlisted_path"
      continue
    fi
    if grep -Fxq "$allowlisted_path" "$INDEX_CONTRACTS_TMP" 2>/dev/null; then
      fail "allowlist entry is stale (already catalogued in INDEX.md): $allowlisted_path"
    fi
  done < "$ALLOWLIST_ENTRIES_TMP"
fi
echo ""

# ── Summary ──

echo "=== Summary ==="
echo "Failures: $failures"
echo "Warnings: $warnings"

if [[ "$failures" -gt 0 ]]; then
  echo ""
  echo "Contract compatibility check failed."
  exit 1
fi

echo ""
echo "Contract compatibility check passed."
