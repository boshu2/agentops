#!/usr/bin/env bash
# Live CLI primitive: --output-schema honors this test-owned exact JSON contract.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=tests/codex/integration/test-helpers.sh
source "$SCRIPT_DIR/test-helpers.sh"
codex_test_setup structured
OUTPUT="$CODEX_TEST_ARTIFACTS/last-message.json"
SCHEMA="$SCRIPT_DIR/../fixtures/structured-output.schema.json"
codex_test_run structured codex exec "${CODEX_TEST_MODEL_ARGS[@]}" -s read-only -C "$CODEX_TEST_PROJECT" \
    --output-schema "$SCHEMA" -o "$OUTPUT" \
    'Return an object with status "ok" and count 3, following the provided schema. No tools are needed.'
codex_test_require_output "$OUTPUT"
# Complete validation of the small fixed schema: exact keys, types and enums.
# Slurp also rejects a stream containing multiple otherwise valid JSON objects.
if ! jq -es 'length == 1 and (.[0] | type == "object" and keys == ["count", "status"] and .status == "ok" and (.count | type == "number") and .count == 3)' "$OUTPUT" >/dev/null; then
    echo "FAIL: output does not satisfy structured-output.schema.json: $OUTPUT" >&2
    exit 1
fi
echo "PASS: --output-schema produced one object satisfying the complete test schema"
