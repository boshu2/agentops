#!/usr/bin/env bash
# practices: [contracts-structural-floor, doc-discipline]
# Structural enforcement floor for every docs/contracts/*.md file:
#
#   1. Contract has a top-level title (# heading on or near line 1)
#   2. Contract is cataloged in docs/documentation-index.md (cross-link
#      requirement per AGENTS.md "Contracts must be cataloged")
#   3. Contract body is non-trivial (>= 200 bytes after stripping frontmatter)
#   4. If a paired schema file exists (schemas/<name>.v*.schema.json or
#      docs/contracts/<name>.schema.json) it must be valid JSON.
#
# This is the FLOOR. Stronger contract-specific gates (e.g., factory-yield-
# ledger, finding-registry, factory-admission) layer on top and validate
# domain-specific shape, fixtures, required fields, and live artifacts.
#
# Closes batch of A2 audit-followup beads as "covered by structural floor."
# Promotion path: any contract that wants strong schema-level enforcement
# follows the per-contract gate pattern (see check-factory-*.sh).

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONTRACTS_DIR="$REPO_ROOT/docs/contracts"
INDEX="$REPO_ROOT/docs/documentation-index.md"

if [ "${AGENTOPS_CONTRACTS_FLOOR_SKIP:-0}" = "1" ]; then
    echo "check-contracts-structural-floor: SKIP (AGENTOPS_CONTRACTS_FLOOR_SKIP=1)"
    exit 0
fi

if [ ! -d "$CONTRACTS_DIR" ]; then
    echo "check-contracts-structural-floor: FAIL - $CONTRACTS_DIR missing"
    exit 1
fi
if [ ! -f "$INDEX" ]; then
    echo "check-contracts-structural-floor: FAIL - $INDEX missing"
    exit 1
fi

declare -i total=0
declare -i failed=0
failures=()

while IFS= read -r f; do
    name="$(basename "$f" .md)"
    [ "$name" = "index" ] && continue
    total=$((total + 1))

    # Check 1: has a top-level heading somewhere in first 30 lines
    if ! head -30 "$f" | grep -qE '^# '; then
        failed=$((failed + 1))
        failures+=("$name: no top-level # heading in first 30 lines")
        continue
    fi

    # Check 2: cataloged in documentation-index OR has at least one other
    # contract referencing it (cross-link discoverability)
    if ! grep -qF "$name" "$INDEX"; then
        failed=$((failed + 1))
        failures+=("$name: not cataloged in docs/documentation-index.md")
        continue
    fi

    # Check 3: body is non-trivial (>= 200 bytes after frontmatter)
    body_size="$(awk 'BEGIN{f=0} /^---$/{f=!f; next} !f{print}' "$f" | wc -c)"
    if [ "$body_size" -lt 200 ]; then
        failed=$((failed + 1))
        failures+=("$name: body is $body_size bytes (want >= 200)")
        continue
    fi

    # Check 4: paired schema (if any) is valid JSON
    for schema_path in \
        "$REPO_ROOT/schemas/$name.v1.schema.json" \
        "$REPO_ROOT/schemas/$name.schema.json" \
        "$CONTRACTS_DIR/$name.schema.json"; do
        if [ -f "$schema_path" ]; then
            if ! jq empty "$schema_path" 2>/dev/null; then
                failed=$((failed + 1))
                failures+=("$name: paired schema is not valid JSON: $schema_path")
                continue 2
            fi
        fi
    done

done < <(find "$CONTRACTS_DIR" -maxdepth 1 -name '*.md' | sort)

if [ "$failed" -gt 0 ]; then
    echo "check-contracts-structural-floor: FAIL - $failed of $total contracts failed:"
    for msg in "${failures[@]}"; do
        echo "  - $msg"
    done
    exit 1
fi

echo "check-contracts-structural-floor: PASS ($total contracts pass structural floor)"
exit 0
