#!/usr/bin/env bash
# practices: [hexagonal-architecture, data-contracts, design-by-contract]
# Enforce that the hook lease inventory is generated from hooks/hooks.json
# and every manifest hook has a disposition for the AgentOps 3.0 migration.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GENERATOR="$REPO_ROOT/scripts/generate-hook-lease-inventory.py"
INVENTORY="$REPO_ROOT/docs/contracts/hook-lease-inventory.md"
SCHEMA="$REPO_ROOT/schemas/hook-lease.v1.schema.json"

if [ "${AGENTOPS_HOOK_LEASE_SKIP:-0}" = "1" ]; then
    echo "check-hook-lease-inventory: SKIP (AGENTOPS_HOOK_LEASE_SKIP=1)"
    exit 0
fi

for f in "$GENERATOR" "$INVENTORY" "$SCHEMA" "$REPO_ROOT/hooks/hooks.json"; do
    if [ ! -f "$f" ]; then
        echo "check-hook-lease-inventory: FAIL - missing file: $f"
        exit 1
    fi
done

if ! jq empty "$SCHEMA" 2>/dev/null; then
    echo "check-hook-lease-inventory: FAIL - schema is not valid JSON: $SCHEMA"
    exit 1
fi

json_tmp="$(mktemp)"
md_tmp="$(mktemp)"
trap 'rm -f "$json_tmp" "$md_tmp"' EXIT

if ! python3 "$GENERATOR" --format json --output "$json_tmp"; then
    exit 1
fi

if ! jq empty "$json_tmp" 2>/dev/null; then
    echo "check-hook-lease-inventory: FAIL - generator emitted invalid JSON"
    exit 1
fi

manifest_count="$(jq '[.hooks | to_entries[] | .value[]?.hooks[]?] | length' "$REPO_ROOT/hooks/hooks.json")"
inventory_count="$(jq '.hook_count' "$json_tmp")"
if [ "$manifest_count" != "$inventory_count" ]; then
    echo "check-hook-lease-inventory: FAIL - hook_count=$inventory_count manifest_count=$manifest_count"
    exit 1
fi

unclassified="$(jq '[.entries[] | select(.disposition == null or .disposition == "")] | length' "$json_tmp")"
if [ "$unclassified" != "0" ]; then
    echo "check-hook-lease-inventory: FAIL - $unclassified unclassified hook entries"
    exit 1
fi

python3 "$GENERATOR" --format markdown --output "$md_tmp" || exit 1
if ! cmp -s "$md_tmp" "$INVENTORY"; then
    echo "check-hook-lease-inventory: FAIL - inventory is stale; run:"
    echo "  python3 scripts/generate-hook-lease-inventory.py --output docs/contracts/hook-lease-inventory.md"
    exit 1
fi

echo "check-hook-lease-inventory: PASS ($inventory_count manifest hook entries classified)"
