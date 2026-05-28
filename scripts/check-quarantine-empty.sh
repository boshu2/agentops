#!/usr/bin/env bash
# check-quarantine-empty.sh — Goal gate script (GOALS.md directive D3)
# Fails when tests/_quarantine/ holds any executable suite (*.sh or *.bats).
# Quarantine is an inbox, not a parking lot. Empty it or document the breakage
# in a tracked bead; do not let the directory accumulate dead tests.
#
# Override (single-cycle, explicit operator opt-out):
#   ALLOW_QUARANTINE=1 bash scripts/check-quarantine-empty.sh
#
# Exit 0 = pass (empty or override), exit 1 = fail (populated without override).
set -euo pipefail

ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || { echo "Not in a git repo"; exit 1; }
QDIR="$ROOT/tests/_quarantine"

if [[ "${ALLOW_QUARANTINE:-0}" == "1" ]]; then
    echo "SKIP: ALLOW_QUARANTINE=1 — quarantine bypass enabled (operator override)"
    exit 0
fi

if [[ ! -d "$QDIR" ]]; then
    echo "PASS: tests/_quarantine/ does not exist (nothing to enforce)"
    exit 0
fi

# Portable enumeration: -print0 + xargs -0 avoids subshell pipe edge cases.
mapfile -d '' FOUND < <(find "$QDIR" -type f \( -name '*.sh' -o -name '*.bats' \) -print0 2>/dev/null)
COUNT=${#FOUND[@]}

if [[ "$COUNT" -gt 0 ]]; then
    echo "FAIL: tests/_quarantine/ holds $COUNT shell/bats suite(s):"
    for f in "${FOUND[@]}"; do
        printf '  - %s\n' "${f#$ROOT/}"
    done
    echo
    echo "Move them back into the tree, delete them, or set ALLOW_QUARANTINE=1 for a single-cycle override."
    exit 1
fi

echo "PASS: tests/_quarantine/ contains no .sh or .bats suites"
exit 0
