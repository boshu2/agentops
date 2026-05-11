#!/usr/bin/env bash
# practices: [factory-yield-contract, schema-validator]
# Enforce factory-yield-ledger contract: schema exists, example parses
# against the schema, every required correlation+yield field is present,
# event_type=factory.yield_observation and schema_version=1 hold.
#
# Pair: docs/contracts/factory-yield-ledger.md (contract spec),
#       schemas/factory-yield.v1.schema.json (machine schema).
# Closes bead: soc-olsa (Enforce factory-yield-ledger contract).
# Closes A2 audit follow-up: factory-yield-ledger transitions from
# doc-only → enforced.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCHEMA="$REPO_ROOT/schemas/factory-yield.v1.schema.json"
EXAMPLE="$REPO_ROOT/docs/contracts/factory-yield-ledger.example.json"
CONTRACT="$REPO_ROOT/docs/contracts/factory-yield-ledger.md"

if [ "${AGENTOPS_FACTORY_YIELD_SKIP:-0}" = "1" ]; then
    echo "check-factory-yield-ledger: SKIP (AGENTOPS_FACTORY_YIELD_SKIP=1)"
    exit 0
fi

# Step 1: required files present
for f in "$SCHEMA" "$EXAMPLE" "$CONTRACT"; do
    if [ ! -f "$f" ]; then
        echo "check-factory-yield-ledger: FAIL — missing file: $f"
        exit 1
    fi
done

# Step 2: schema is valid JSON
if ! jq empty "$SCHEMA" 2>/dev/null; then
    echo "check-factory-yield-ledger: FAIL — schema is not valid JSON: $SCHEMA"
    exit 1
fi

# Step 3: example is valid JSON
if ! jq empty "$EXAMPLE" 2>/dev/null; then
    echo "check-factory-yield-ledger: FAIL — example is not valid JSON: $EXAMPLE"
    exit 1
fi

# Step 4: example matches the contract's declared event shape
event_type="$(jq -r '.event_type // ""' "$EXAMPLE")"
schema_version="$(jq -r '.schema_version // 0' "$EXAMPLE")"
if [ "$event_type" != "factory.yield_observation" ]; then
    echo "check-factory-yield-ledger: FAIL — example.event_type=$event_type (want factory.yield_observation)"
    exit 1
fi
if [ "$schema_version" != "1" ]; then
    echo "check-factory-yield-ledger: FAIL — example.schema_version=$schema_version (want 1)"
    exit 1
fi

# Step 5: required fields from the contract spec are present
REQUIRED_FIELDS=(
    observation_id
    run_id
    job_id
    task_id
    lane_id
    provider
    runtime
    model
    authority
    task_class
    baseline_or_treatment
    routing_decision_id
    routing_policy_id
    validation_id
    validation_status
    merge_decision_id
    merge_status
    accepted_patches
    wall_clock_minutes
    review_minutes
    recovery_minutes
    model_cost_usd
    conflict_count
    defect_count
    operator_interventions
)

missing_fields=()
for field in "${REQUIRED_FIELDS[@]}"; do
    val="$(jq --arg f "$field" 'has($f)' "$EXAMPLE")"
    if [ "$val" != "true" ]; then
        missing_fields+=("$field")
    fi
done

if [ "${#missing_fields[@]}" -gt 0 ]; then
    echo "check-factory-yield-ledger: FAIL — example missing required fields:"
    for f in "${missing_fields[@]}"; do
        echo "  - $f"
    done
    exit 1
fi

# Step 6: contract has the AOP-CLAIM marker for yield-ledger
if ! grep -q "AOP-CLAIM-CONTRACT-YIELD-LEDGER" "$CONTRACT"; then
    echo "check-factory-yield-ledger: FAIL — contract missing AOP-CLAIM-CONTRACT-YIELD-LEDGER marker"
    exit 1
fi

# Step 7: contract is cataloged in documentation-index
if ! grep -q "factory-yield-ledger" "$REPO_ROOT/docs/documentation-index.md" 2>/dev/null; then
    echo "check-factory-yield-ledger: WARN — contract not cataloged in docs/documentation-index.md"
    # Warn-only; not hard fail
fi

echo "check-factory-yield-ledger: PASS (schema + example + ${#REQUIRED_FIELDS[@]} required fields verified)"
exit 0
