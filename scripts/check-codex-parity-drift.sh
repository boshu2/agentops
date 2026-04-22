#!/usr/bin/env bash
# check-codex-parity-drift.sh — Goal gate script
# Runs audit-codex-parity.py and fails if any findings exist.
# Exit 0 = pass (no drift), exit 1 = fail (drift detected)
set -euo pipefail

ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || { echo "Not in a git repo"; exit 1; }

AUDIT_PY="$ROOT/scripts/audit-codex-parity.py"
if [[ ! -f "$AUDIT_PY" ]]; then
    echo "SKIP: $AUDIT_PY not found"
    exit 0
fi

# Run audit in JSON mode. The script exits 0 when clean, 1 when drift exists.
# We always want to see findings on failure, so capture output and status separately.
JSON_OUTPUT=$(python3 "$AUDIT_PY" --repo-root "$ROOT" --json 2>/dev/null || true)
STATUS=0
python3 "$AUDIT_PY" --repo-root "$ROOT" >/dev/null 2>&1 || STATUS=$?

if [[ "$STATUS" -ne 0 ]]; then
    FINDING_COUNT=$(python3 -c "import json,sys; print(len(json.loads(sys.argv[1] or '[]')))" "$JSON_OUTPUT" 2>/dev/null || echo "?")
    echo "FAIL: $FINDING_COUNT codex parity finding(s) detected"
    python3 "$AUDIT_PY" --repo-root "$ROOT" 2>&1 | head -40 || true
    exit 1
fi

echo "PASS: No codex parity drift detected"
exit 0
