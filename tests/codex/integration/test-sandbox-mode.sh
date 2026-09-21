#!/usr/bin/env bash
# Live CLI primitive: read-only sandbox argument and CLI-level final output capture.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=tests/codex/integration/test-helpers.sh
source "$SCRIPT_DIR/test-helpers.sh"
codex_test_setup sandbox
OUTPUT="$CODEX_TEST_ARTIFACTS/last-message.txt"
printf 'Fixture contains one read-only probe file. No project changes are requested.\n' > "$CODEX_TEST_PROJECT/README.md"
codex_test_run sandbox codex exec "${CODEX_TEST_MODEL_ARGS[@]}" -s read-only -C "$CODEX_TEST_PROJECT" \
    -o "$OUTPUT" "Read README.md and summarize its contents in one complete sentence. Do not modify files."
codex_test_require_output "$OUTPUT"
SIZE=$(wc -c < "$OUTPUT" | tr -d ' ')
if [[ "$SIZE" -le 50 ]]; then
    echo "FAIL: read-only output has fewer than 51 bytes ($SIZE)" >&2
    exit 1
fi
echo "PASS: codex exec -s read-only completed and -o captured substantive output ($SIZE bytes)"
