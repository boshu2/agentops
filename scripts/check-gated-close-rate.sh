#!/usr/bin/env bash
# practices: [wiki-knowledge-surface, resilience-patterns, ai-assisted-dev]
# check-gated-close-rate.sh — fitness gate for the close-admission discipline (cp-m8md, kin of cp-irmu).
#
# Of the last N closes in the control-plane br ledger, what fraction carry the
# close-admission gate stamp ('close-admission gate PASS') in their close_reason?
# A high rate means closes are going through the gate, not around it.
#
# This gate reads control-plane's ledger, which is OPTIONAL from agentops's POV:
#   - CONTROL_PLANE_ROOT env override (default /Users/bo/dev/control-plane)
#   - if br is unavailable OR the CP root is absent, SKIP cleanly (exit 0) with a notice
#     — mirrors the house optional-dependency pattern (see check-corpus-freshness.sh).
#   - SKIP=1 (AGENTOPS_GATED_CLOSE_RATE_SKIP=1) short-circuits with PASS.
#
# The ledger read is strictly READ-ONLY (br list --status closed --json).
#
# Threshold: default 70 (percent). Measured 2026-06-10 = 80% over last 20 closes.
# Override: --threshold N (percent) or AGENTOPS_GATED_CLOSE_RATE_THRESHOLD.
# Window:   default 20 closes. Override: --window N or AGENTOPS_GATED_CLOSE_RATE_WINDOW.
#
# Exit codes: 0 PASS or clean SKIP; 1 FAIL (rate below threshold).
#
# Usage: bash scripts/check-gated-close-rate.sh [--threshold N] [--window N] [--json]

set -uo pipefail

CP_ROOT="${CONTROL_PLANE_ROOT:-/Users/bo/dev/control-plane}"
THRESHOLD="${AGENTOPS_GATED_CLOSE_RATE_THRESHOLD:-70}"
WINDOW="${AGENTOPS_GATED_CLOSE_RATE_WINDOW:-20}"
STAMP='close-admission gate PASS'
JSON=0

while [ $# -gt 0 ]; do
  case "$1" in
    --threshold) shift; THRESHOLD="${1:?--threshold needs a value}" ;;
    --window)    shift; WINDOW="${1:?--window needs a value}" ;;
    --json)      JSON=1 ;;
    -h|--help)   grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "check-gated-close-rate: unknown arg: $1" >&2; exit 2 ;;
  esac
  shift
done

# Operator override
if [ "${AGENTOPS_GATED_CLOSE_RATE_SKIP:-0}" = "1" ]; then
  echo "check-gated-close-rate: SKIP (AGENTOPS_GATED_CLOSE_RATE_SKIP=1)"
  exit 0
fi

# Optional cross-repo dependency: br CLI + control-plane root must both be present.
if ! command -v br >/dev/null 2>&1; then
  echo "check-gated-close-rate: SKIP (br CLI not on PATH — close-admission ledger unreadable here)"
  exit 0
fi
if [ ! -d "$CP_ROOT" ]; then
  echo "check-gated-close-rate: SKIP (control-plane root absent at $CP_ROOT — set CONTROL_PLANE_ROOT)"
  exit 0
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "check-gated-close-rate: SKIP (jq not on PATH)"
  exit 0
fi

# READ-ONLY ledger read, run from the CP root so br resolves the right workspace.
CLOSED_JSON=$( (cd "$CP_ROOT" && br list --status closed --json 2>/dev/null) )
if [ -z "$CLOSED_JSON" ]; then
  echo "check-gated-close-rate: SKIP (br returned no closed-issue JSON from $CP_ROOT)"
  exit 0
fi

TOTAL=$(printf '%s' "$CLOSED_JSON" | jq --argjson n "$WINDOW" \
  '.issues | sort_by(.closed_at) | reverse | .[0:$n] | length' 2>/dev/null)
if [ -z "$TOTAL" ] || [ "$TOTAL" = "null" ] || [ "$TOTAL" -eq 0 ] 2>/dev/null; then
  echo "check-gated-close-rate: SKIP (no closed beads in window from $CP_ROOT)"
  exit 0
fi

GATED=$(printf '%s' "$CLOSED_JSON" | jq --argjson n "$WINDOW" --arg stamp "$STAMP" \
  '.issues | sort_by(.closed_at) | reverse | .[0:$n]
   | map(select(.close_reason // "" | contains($stamp))) | length' 2>/dev/null)

RATE=$(( GATED * 100 / TOTAL ))

if [ "$JSON" -eq 1 ]; then
  result=$([ "$RATE" -ge "$THRESHOLD" ] && echo PASS || echo FAIL)
  printf '{"gated":%d,"total":%d,"rate_pct":%d,"threshold_pct":%d,"window":%d,"result":"%s"}\n' \
    "$GATED" "$TOTAL" "$RATE" "$THRESHOLD" "$WINDOW" "$result"
fi

if [ "$RATE" -lt "$THRESHOLD" ]; then
  [ "$JSON" -eq 1 ] || echo "check-gated-close-rate: FAIL — ${GATED}/${TOTAL} of last $WINDOW closes carry the gate stamp = ${RATE}% (< ${THRESHOLD}% threshold)"
  [ "$JSON" -eq 1 ] || echo "  fix: route closes through ctl-close-admission so close_reason carries 'close-admission gate PASS'"
  exit 1
fi

[ "$JSON" -eq 1 ] || echo "check-gated-close-rate: PASS — ${GATED}/${TOTAL} of last $WINDOW closes gated = ${RATE}% (>= ${THRESHOLD}% threshold)"
exit 0
