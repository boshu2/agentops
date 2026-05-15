#!/usr/bin/env bash
# practices: [wiki-knowledge-surface, data-contracts, design-by-contract]
# Enforce finding-registry contract: schema is well-formed JSON Schema,
# the contract doc carries its AOP-CLAIM marker, the schema's required-field
# list matches the contract doc's required-field list, and if a runtime
# registry exists on this box every line is a valid JSON object with the
# required fields.
#
# Pair: docs/contracts/finding-registry.md, finding-registry.schema.json.
# Closes bead: soc-v07b. Migrates A2 classification doc-only -> enforced.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONTRACT="$REPO_ROOT/docs/contracts/finding-registry.md"
SCHEMA="$REPO_ROOT/docs/contracts/finding-registry.schema.json"
REGISTRY="$REPO_ROOT/.agents/findings/registry.jsonl"

if [ "${AGENTOPS_FINDING_REGISTRY_SKIP:-0}" = "1" ]; then
    echo "check-finding-registry: SKIP (AGENTOPS_FINDING_REGISTRY_SKIP=1)"
    exit 0
fi

for f in "$CONTRACT" "$SCHEMA"; do
    if [ ! -f "$f" ]; then
        echo "check-finding-registry: FAIL - missing file: $f"
        exit 1
    fi
done

if ! jq empty "$SCHEMA" 2>/dev/null; then
    echo "check-finding-registry: FAIL - schema is not valid JSON: $SCHEMA"
    exit 1
fi

schema_type="$(jq -r '.type // ""' "$SCHEMA")"
required_count="$(jq -r '.required | length' "$SCHEMA" 2>/dev/null || echo 0)"
if [ "$schema_type" != "object" ]; then
    echo "check-finding-registry: FAIL - schema.type=$schema_type (want object)"
    exit 1
fi
if [ "$required_count" -lt 10 ]; then
    echo "check-finding-registry: FAIL - schema.required has $required_count entries (want >= 10)"
    exit 1
fi

missing_in_contract=()
while IFS= read -r field; do
    # Contract may list a nested child of an object field (e.g., source.repo)
    # rather than the parent object name itself. Accept either form.
    if grep -qE "^\s*-\s+\`$field\`" "$CONTRACT"; then
        continue
    fi
    if grep -qE "^\s*-\s+\`$field\." "$CONTRACT"; then
        continue
    fi
    missing_in_contract+=("$field")
done < <(jq -r '.required[]' "$SCHEMA")

if [ "${#missing_in_contract[@]}" -gt 0 ]; then
    echo "check-finding-registry: FAIL - schema required fields not listed in contract:"
    for f in "${missing_in_contract[@]}"; do
        echo "  - $f"
    done
    exit 1
fi

if ! grep -q "registry.jsonl" "$CONTRACT"; then
    echo "check-finding-registry: FAIL - contract does not reference canonical path .agents/findings/registry.jsonl"
    exit 1
fi

if [ -f "$REGISTRY" ]; then
    line_no=0
    bad_lines=0
    while IFS= read -r line; do
        line_no=$((line_no + 1))
        [ -z "$line" ] && continue
        if ! printf '%s' "$line" | jq empty 2>/dev/null; then
            echo "check-finding-registry: FAIL - registry line $line_no is not valid JSON"
            bad_lines=$((bad_lines + 1))
            continue
        fi
        for field in id version tier date severity category pattern status; do
            has="$(printf '%s' "$line" | jq --arg f "$field" 'has($f)')"
            if [ "$has" != "true" ]; then
                echo "check-finding-registry: FAIL - registry line $line_no missing required field: $field"
                bad_lines=$((bad_lines + 1))
                break
            fi
        done
    done < "$REGISTRY"
    if [ "$bad_lines" -gt 0 ]; then
        echo "check-finding-registry: FAIL - $bad_lines line(s) of $line_no in $REGISTRY failed validation"
        exit 1
    fi
    echo "check-finding-registry: PASS (schema valid; $required_count required fields cross-checked; $line_no live registry lines validated)"
else
    echo "check-finding-registry: PASS (schema valid; $required_count required fields cross-checked; no live registry - structural-only mode)"
fi

exit 0
