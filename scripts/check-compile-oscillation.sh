#!/usr/bin/env bash
# check-compile-oscillation.sh — Gate: No evolve goals are oscillating in the
# most recent defrag report.
#
# Exit 0 = PASS, Exit 1 = FAIL
#
# Looks at .agents/defrag/latest.json first. When missing and COMPILE_OUTPUT_DIR
# is unset, falls back to the freshest Dream overnight preview at
# .agents/overnight/<run>/defrag/latest.json (same shape). This keeps the gate
# green on machines that have run Dream but not a manual `ao defrag`.
set -euo pipefail

AGENTS_DIR="${AGENTS_DIR:-.agents}"
DEFRAG_LATEST="${COMPILE_OUTPUT_DIR:-$AGENTS_DIR}/defrag/latest.json"

if [[ ! -f "$DEFRAG_LATEST" && -z "${COMPILE_OUTPUT_DIR:-}" ]]; then
    overnight_root="$AGENTS_DIR/overnight"
    if [[ -d "$overnight_root" ]]; then
        fallback="$(find "$overnight_root" -path '*/defrag/latest.json' -type f -printf '%T@ %p\n' 2>/dev/null \
            | sort -n | tail -n 1 | awk '{print $2}')"
        if [[ -n "$fallback" && -f "$fallback" ]]; then
            echo "INFO: $DEFRAG_LATEST not found; falling back to overnight preview $fallback"
            DEFRAG_LATEST="$fallback"
        fi
    fi
fi

if [[ ! -f "$DEFRAG_LATEST" ]]; then
    echo "FAIL: $DEFRAG_LATEST not found — run 'ao defrag --oscillation-sweep' first"
    exit 1
fi

if ! jq -e "(.oscillation.oscillating_goals // []) | length == 0" "$DEFRAG_LATEST" >/dev/null 2>&1; then
    count=$(jq -r "(.oscillation.oscillating_goals // []) | length" "$DEFRAG_LATEST" 2>/dev/null || echo "?")
    echo "FAIL: $count oscillating goal(s) in $DEFRAG_LATEST"
    exit 1
fi

echo "PASS: no oscillating goals in $DEFRAG_LATEST"
exit 0
