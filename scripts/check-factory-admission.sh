#!/usr/bin/env bash
# practices: [data-contracts, design-by-contract, microservices]
# Enforce factory-admission contract: wraps the existing
# tests/scripts/test-factory-admission-contracts.py so it runs as a
# blocking gate instead of as an orphaned test fixture.
#
# Pair: docs/contracts/factory-admission.md, schemas/factory-admission.
# v1.schema.json, tests/fixtures/factory-admission/valid-*.json.
# Closes bead: soc-2cy7. Migrates A2 classification partial -> enforced.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEST_SCRIPT="$REPO_ROOT/tests/scripts/test-factory-admission-contracts.py"
SCHEMA="$REPO_ROOT/schemas/factory-admission.v1.schema.json"
CONTRACT="$REPO_ROOT/docs/contracts/factory-admission.md"

if [ "${AGENTOPS_FACTORY_ADMISSION_SKIP:-0}" = "1" ]; then
    echo "check-factory-admission: SKIP (AGENTOPS_FACTORY_ADMISSION_SKIP=1)"
    exit 0
fi

for f in "$TEST_SCRIPT" "$SCHEMA" "$CONTRACT"; do
    if [ ! -f "$f" ]; then
        echo "check-factory-admission: FAIL - missing file: $f"
        exit 1
    fi
done

if ! command -v python3 >/dev/null 2>&1; then
    echo "check-factory-admission: SKIP (python3 not available)"
    exit 0
fi

# Run the existing Python test; require jsonschema to be importable.
if ! python3 -c "import jsonschema" 2>/dev/null; then
    echo "check-factory-admission: SKIP (jsonschema package not installed; pip install jsonschema)"
    exit 0
fi

if ! out="$(python3 "$TEST_SCRIPT" 2>&1)"; then
    echo "check-factory-admission: FAIL"
    echo "$out" | head -20
    exit 1
fi

echo "check-factory-admission: PASS (schema + fixtures validated via tests/scripts/test-factory-admission-contracts.py)"
exit 0
