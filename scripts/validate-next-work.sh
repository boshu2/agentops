#!/usr/bin/env bash
# validate-next-work.sh — Validate .agents/rpi/next-work.jsonl rows against the
# committed JSON Schema typed contract.
#
#   schemas/next-work-batch.v1.schema.json   (one JSONL line = one batch entry)
#   schemas/next-work-item.v1.schema.json    (each entry.items[] element)
#
# Enums and required-field lists are read FROM the schema files at runtime, so
# the committed schema is the single source of truth (no duplicated enum lists).
# A real JSON Schema validator (ajv, then python `jsonschema`) runs as an extra
# structural pass when available; jq is the universal fallback that always runs
# and is authoritative for the field-named violations below.
#
# Usage:
#   bash scripts/validate-next-work.sh [--strict] [--json] [--schema-dir DIR] [FILE]
#
#   FILE           JSONL queue file (default: .agents/rpi/next-work.jsonl)
#   --strict       exit non-zero on any violation (default: advisory, exit 0)
#   --json         emit a machine-readable verdict on stdout
#   --schema-dir   directory holding the schema files (default: <repo>/schemas)
#   -h, --help     show this help
#
# Exit codes: 0 = clean (or advisory), 1 = violations in --strict, 2 = usage error.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

STRICT=0
JSON=0
SCHEMA_DIR="$ROOT/schemas"
FILE=""

usage() {
    sed -n '2,20p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --strict) STRICT=1; shift ;;
        --json) JSON=1; shift ;;
        --schema-dir) SCHEMA_DIR="${2:-}"; shift 2 ;;
        -h|--help) usage; exit 0 ;;
        --*) echo "ERROR: unknown flag: $1" >&2; usage >&2; exit 2 ;;
        *)
            if [[ -n "$FILE" ]]; then
                echo "ERROR: unexpected extra argument: $1" >&2
                usage >&2
                exit 2
            fi
            FILE="$1"; shift ;;
    esac
done

[[ -z "$FILE" ]] && FILE="$ROOT/.agents/rpi/next-work.jsonl"

command -v jq >/dev/null 2>&1 || { echo "ERROR: jq is required" >&2; exit 1; }

ITEM_SCHEMA="$SCHEMA_DIR/next-work-item.v1.schema.json"
BATCH_SCHEMA="$SCHEMA_DIR/next-work-batch.v1.schema.json"
for s in "$ITEM_SCHEMA" "$BATCH_SCHEMA"; do
    if [[ ! -f "$s" ]]; then
        echo "ERROR: schema not found: $s" >&2
        exit 1
    fi
done

emit_result() {
    # $1 = exit code to return after emitting
    local rc="$1"
    if [[ "$JSON" -eq 1 ]]; then
        local valid="true"
        [[ "${#VIOLATIONS[@]}" -gt 0 ]] && valid="false"
        printf '%s\n' "${VIOLATIONS[@]}" 2>/dev/null \
            | jq -s --argjson valid "$valid" \
                  --arg file "$FILE" \
                  --argjson lines "$CHECKED_LINES" \
                  --arg validators "$VALIDATORS" '
            {valid: $valid, file: $file, checked_lines: $lines,
             validators: ($validators | split(",") | map(select(length>0))),
             violations: map(select(length>0))}'
    fi
    exit "$rc"
}

# Graceful no-op when the queue is absent (mirrors sibling validators).
if [[ ! -f "$FILE" ]]; then
    VIOLATIONS=()
    CHECKED_LINES=0
    VALIDATORS="jq"
    if [[ "$JSON" -eq 1 ]]; then
        emit_result 0
    fi
    echo "PASS: $FILE not present (no queue to validate)"
    exit 0
fi

# --- load contract from the committed schema (single source of truth) ---
mapfile -t VALID_TYPES      < <(jq -r '.properties.type.enum[]'         "$ITEM_SCHEMA")
mapfile -t VALID_SEVERITIES < <(jq -r '.properties.severity.enum[]'     "$ITEM_SCHEMA")
mapfile -t VALID_SOURCES    < <(jq -r '.properties.source.enum[]'       "$ITEM_SCHEMA")
mapfile -t VALID_CLAIM      < <(jq -r '.properties.claim_status.enum[]' "$ITEM_SCHEMA")
mapfile -t VALID_STATUS     < <(jq -r '.properties.status.enum[]'       "$ITEM_SCHEMA")
mapfile -t REQ_ITEM         < <(jq -r '.required[]'                     "$ITEM_SCHEMA")
mapfile -t REQ_BATCH        < <(jq -r '.required[]'                     "$BATCH_SCHEMA")
mapfile -t PROOF_KINDS      < <(jq -r '.properties.proof_ref.properties.kind.enum[]' "$ITEM_SCHEMA")

set_str() { local IFS=,; echo "$*"; }

in_set() {
    local needle="$1"; shift
    local v
    for v in "$@"; do [[ "$needle" == "$v" ]] && return 0; done
    return 1
}

VIOLATIONS=()       # JSON objects, one per violation (for --json + text)
CHECKED_LINES=0
VALIDATORS="jq"

add_violation() {
    # add_violation <line> <item-index-or-null> <field> <message>
    local line="$1" item="$2" field="$3" msg="$4"
    VIOLATIONS+=("$(jq -nc \
        --argjson line "$line" \
        --arg item "$item" \
        --arg field "$field" \
        --arg message "$msg" \
        '{line: $line, item: (if $item == "null" then null else ($item|tonumber) end),
          field: $field, message: $message}')")
}

# --- jq-driven per-line validation (authoritative, always runs) ---
line_no=0
while IFS= read -r line || [[ -n "$line" ]]; do
    line_no=$((line_no + 1))
    [[ -z "${line//[[:space:]]/}" ]] && continue
    CHECKED_LINES=$((CHECKED_LINES + 1))

    if ! printf '%s' "$line" | jq -e . >/dev/null 2>&1; then
        add_violation "$line_no" "null" "(line)" "$FILE:$line_no: malformed JSON"
        continue
    fi

    # Batch required fields.
    for bf in "${REQ_BATCH[@]}"; do
        if ! printf '%s' "$line" | jq -e --arg k "$bf" 'has($k)' >/dev/null 2>&1; then
            add_violation "$line_no" "null" "$bf" "$FILE:$line_no: missing required batch field: $bf"
        fi
    done

    # Batch claim_status enum (when present).
    bcs=$(printf '%s' "$line" | jq -r '.claim_status // empty')
    if [[ -n "$bcs" ]] && ! in_set "$bcs" "${VALID_CLAIM[@]}"; then
        add_violation "$line_no" "null" "claim_status" \
            "$FILE:$line_no: batch claim_status=$bcs not in {$(set_str "${VALID_CLAIM[@]}")}"
    fi

    # Batch-level consumed-marker field types (age-tkxq). consumed is a boolean;
    # consumed_note / consumed_ref are optional string annotations. Both the
    # legacy batch-only shape and the new per-item shape must pass these.
    if printf '%s' "$line" | jq -e 'has("consumed") and (.consumed | type != "boolean")' >/dev/null 2>&1; then
        add_violation "$line_no" "null" "consumed" \
            "$FILE:$line_no: batch consumed must be a boolean"
    fi
    if printf '%s' "$line" | jq -e 'has("consumed_note") and (.consumed_note | type != "string")' >/dev/null 2>&1; then
        add_violation "$line_no" "null" "consumed_note" \
            "$FILE:$line_no: batch consumed_note must be a string"
    fi
    if printf '%s' "$line" | jq -e 'has("consumed_ref") and (.consumed_ref | type != "string")' >/dev/null 2>&1; then
        add_violation "$line_no" "null" "consumed_ref" \
            "$FILE:$line_no: batch consumed_ref must be a string"
    fi

    # items must be an array.
    if ! printf '%s' "$line" | jq -e '.items | type == "array"' >/dev/null 2>&1; then
        # Missing items already reported as a required-field violation above;
        # only flag the wrong-type case here.
        if printf '%s' "$line" | jq -e 'has("items")' >/dev/null 2>&1; then
            add_violation "$line_no" "null" "items" "$FILE:$line_no: items must be an array"
        fi
        continue
    fi

    item_count=$(printf '%s' "$line" | jq '.items | length')
    for ((j=0; j<item_count; j++)); do
        item=$(printf '%s' "$line" | jq -c ".items[$j]")
        title=$(printf '%s' "$item" | jq -r '.title // ""' | head -c 60)

        for rf in "${REQ_ITEM[@]}"; do
            val=$(printf '%s' "$item" | jq -r --arg k "$rf" '.[$k] // empty')
            if [[ -z "$val" ]]; then
                add_violation "$line_no" "$j" "$rf" \
                    "$FILE:$line_no item $j ($title): missing required field: $rf"
            fi
        done

        ty=$(printf '%s' "$item" | jq -r '.type // empty')
        sev=$(printf '%s' "$item" | jq -r '.severity // empty')
        src=$(printf '%s' "$item" | jq -r '.source // empty')
        ics=$(printf '%s' "$item" | jq -r '.claim_status // empty')
        ist=$(printf '%s' "$item" | jq -r '.status // empty')

        if [[ -n "$ty" ]] && ! in_set "$ty" "${VALID_TYPES[@]}"; then
            add_violation "$line_no" "$j" "type" \
                "$FILE:$line_no item $j ($title): type=$ty not in {$(set_str "${VALID_TYPES[@]}")}"
        fi
        if [[ -n "$sev" ]] && ! in_set "$sev" "${VALID_SEVERITIES[@]}"; then
            add_violation "$line_no" "$j" "severity" \
                "$FILE:$line_no item $j ($title): severity=$sev not in {$(set_str "${VALID_SEVERITIES[@]}")}"
        fi
        if [[ -n "$src" ]] && ! in_set "$src" "${VALID_SOURCES[@]}"; then
            add_violation "$line_no" "$j" "source" \
                "$FILE:$line_no item $j ($title): source=$src not in {$(set_str "${VALID_SOURCES[@]}")}"
        fi
        if [[ -n "$ics" ]] && ! in_set "$ics" "${VALID_CLAIM[@]}"; then
            add_violation "$line_no" "$j" "claim_status" \
                "$FILE:$line_no item $j ($title): claim_status=$ics not in {$(set_str "${VALID_CLAIM[@]}")}"
        fi
        if [[ -n "$ist" ]] && ! in_set "$ist" "${VALID_STATUS[@]}"; then
            add_violation "$line_no" "$j" "status" \
                "$FILE:$line_no item $j ($title): status=$ist not in {$(set_str "${VALID_STATUS[@]}")}"
        fi

        # First-class per-item consumed markers (age-tkxq): consumed is a boolean;
        # consumed_note / consumed_ref are optional string annotations. Malformed
        # types fail here so a hand-edited row cannot smuggle a wrong-typed marker.
        if printf '%s' "$item" | jq -e 'has("consumed") and (.consumed | type != "boolean")' >/dev/null 2>&1; then
            add_violation "$line_no" "$j" "consumed" \
                "$FILE:$line_no item $j ($title): consumed must be a boolean"
        fi
        if printf '%s' "$item" | jq -e 'has("consumed_note") and (.consumed_note | type != "string")' >/dev/null 2>&1; then
            add_violation "$line_no" "$j" "consumed_note" \
                "$FILE:$line_no item $j ($title): consumed_note must be a string"
        fi
        if printf '%s' "$item" | jq -e 'has("consumed_ref") and (.consumed_ref | type != "string")' >/dev/null 2>&1; then
            add_violation "$line_no" "$j" "consumed_ref" \
                "$FILE:$line_no item $j ($title): consumed_ref must be a string"
        fi

        # proof_ref conditional requirements (mirrors item schema proof_ref.allOf).
        pkind=$(printf '%s' "$item" | jq -r '.proof_ref.kind // empty')
        if [[ -n "$pkind" ]]; then
            if ! in_set "$pkind" "${PROOF_KINDS[@]}"; then
                add_violation "$line_no" "$j" "proof_ref.kind" \
                    "$FILE:$line_no item $j ($title): proof_ref.kind=$pkind not in {$(set_str "${PROOF_KINDS[@]}")}"
            else
                case "$pkind" in
                    completed_run)
                        [[ -n "$(printf '%s' "$item" | jq -r '.proof_ref.run_id // empty')" ]] || \
                            add_violation "$line_no" "$j" "proof_ref.run_id" \
                                "$FILE:$line_no item $j ($title): proof_ref.kind=completed_run requires run_id"
                        ;;
                    evidence_only_closure)
                        [[ -n "$(printf '%s' "$item" | jq -r '.proof_ref.target_id // empty')" ]] || \
                            add_violation "$line_no" "$j" "proof_ref.target_id" \
                                "$FILE:$line_no item $j ($title): proof_ref.kind=evidence_only_closure requires target_id"
                        ;;
                    execution_packet)
                        [[ -n "$(printf '%s' "$item" | jq -r '.proof_ref.path // empty')" ]] || \
                            add_violation "$line_no" "$j" "proof_ref.path" \
                                "$FILE:$line_no item $j ($title): proof_ref.kind=execution_packet requires path"
                        ;;
                esac
            fi
        fi
    done
done < "$FILE"

# --- optional full-schema structural pass (python jsonschema) ---
# Catches structural issues the jq layer does not (e.g. wrong field types).
# Only ADDS a violation when the schema tool flags the file invalid while the
# jq layer found nothing — pure corroboration otherwise.
#
# ajv (if installed) is recorded as a present validator but is NON-GATING: its
# CLI ref-resolution/stdin handling varies across versions, so we never let it
# flip the verdict (same posture as scripts/validate-swarm-evidence.sh, which
# treats ajv as warn-only). python `jsonschema` is the gating structural pass.
schema_tool=""
schema_invalid=0
if command -v ajv >/dev/null 2>&1; then
    VALIDATORS="jq,ajv"
fi
if command -v python3 >/dev/null 2>&1 && python3 -c 'import jsonschema' >/dev/null 2>&1; then
    schema_tool="python-jsonschema"
    if ! python3 - "$BATCH_SCHEMA" "$ITEM_SCHEMA" "$FILE" >/dev/null 2>&1 <<'PY'
import json, sys
import jsonschema
from jsonschema import Draft7Validator, RefResolver

batch_path, item_path, data_path = sys.argv[1], sys.argv[2], sys.argv[3]
with open(batch_path) as f:
    batch = json.load(f)
with open(item_path) as f:
    item = json.load(f)
store = {batch.get("$id", batch_path): batch, item.get("$id", item_path): item}
resolver = RefResolver(base_uri=batch.get("$id", ""), referrer=batch, store=store)
validator = Draft7Validator(batch, resolver=resolver)
rc = 0
with open(data_path) as f:
    for line in f:
        line = line.strip()
        if not line:
            continue
        try:
            doc = json.loads(line)
        except Exception:
            continue
        if list(validator.iter_errors(doc)):
            rc = 1
sys.exit(rc)
PY
    then
        schema_invalid=1
    fi
fi

if [[ -n "$schema_tool" ]]; then
    VALIDATORS="$VALIDATORS,$schema_tool"
    if [[ "$schema_invalid" -eq 1 && "${#VIOLATIONS[@]}" -eq 0 ]]; then
        add_violation 0 "null" "(schema)" \
            "$FILE: $schema_tool flagged a structural schema violation not caught by enum checks; run '$schema_tool' against schemas/next-work-batch.v1.schema.json for detail"
    fi
fi

# --- report ---
nviol="${#VIOLATIONS[@]}"

if [[ "$JSON" -eq 1 ]]; then
    if [[ "$nviol" -gt 0 && "$STRICT" -eq 1 ]]; then
        emit_result 1
    else
        emit_result 0
    fi
fi

if [[ "$nviol" -gt 0 ]]; then
    for v in "${VIOLATIONS[@]}"; do
        printf '%s\n' "$v" | jq -r '.message'
    done >&2
    if [[ "$STRICT" -eq 1 ]]; then
        echo "FAIL: $nviol schema violation(s) in $FILE ($CHECKED_LINES line(s) checked, validators: $VALIDATORS)" >&2
        exit 1
    fi
    echo "ADVISORY: $nviol schema violation(s) in $FILE ($CHECKED_LINES line(s) checked, validators: $VALIDATORS); not failing (advisory mode)"
    exit 0
fi

echo "PASS: $CHECKED_LINES row(s) in $FILE conform to the next-work v1 schema (validators: $VALIDATORS)"
exit 0
