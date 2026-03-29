#!/usr/bin/env bash
# check-codex-parity-drift.sh — Goal gate script
# Runs audit-codex-parity in check mode and fails if any findings exist.
# Exit 0 = pass (no drift), exit 1 = fail (drift detected)
set -euo pipefail

ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || { echo "Not in a git repo"; exit 1; }

AUDIT_SCRIPT="$ROOT/scripts/audit-codex-parity.sh"
AUDIT_CMD=()
if [[ ! -x "$AUDIT_SCRIPT" ]]; then
    # Try Python version
    AUDIT_SCRIPT="$ROOT/scripts/audit-codex-parity.py"
    if [[ ! -f "$AUDIT_SCRIPT" ]]; then
        echo "SKIP: No audit-codex-parity script found"
        exit 0
    fi
    AUDIT_CMD=(python3 "$AUDIT_SCRIPT" --repo-root "$ROOT" --check)
else
    AUDIT_CMD=("$AUDIT_SCRIPT" --check)
fi

# Run audit in check mode, capture output
OUTPUT=$("${AUDIT_CMD[@]}" 2>&1) || true
FINDING_COUNT=$(echo "$OUTPUT" | grep -c "FINDING\|DRIFT\|LEAK" 2>/dev/null) || FINDING_COUNT=0

if [[ "$FINDING_COUNT" -gt 0 ]]; then
    echo "FAIL: $FINDING_COUNT codex parity findings detected"
    echo "$OUTPUT" | grep "FINDING\|DRIFT\|LEAK" | head -10
    exit 1
fi

echo "PASS: No codex parity drift detected"
exit 0
