#!/usr/bin/env bash
# practices: [wiki-knowledge-surface, resilience-patterns, ai-assisted-dev]
# check-verdict-close-rate.sh — warn-first fitness gate for verdict-referenced
# bead closes (age-wedge-all-in-dyr0.4, kin of check-gated-close-rate.sh).
#
# Of the last N closes in the br ledger JSONL (issues.jsonl), what fraction
# carry a '[verdict:...]' close-admission stamp in close_reason? The stamp is
# written by `ao done` ("[verdict:<sha7>:<CONFIRMED|waived-trivial|UNVERIFIED>]"),
# so a high rate means closes reference ledger proof instead of prose.
#
# Ledger resolution (no hardcoded paths):
#   - $BEADS_DIR when exported (explicit override — same precedence br itself
#     gives the env), else `ao beads dir`; if neither resolves, SKIP cleanly.
#   - jq absent, or issues.jsonl absent/unreadable => SKIP cleanly (exit 0
#     with a notice) — mirrors the house optional-dependency pattern
#     (see check-provenance-feed-health.sh / check-gated-close-rate.sh).
# The ledger read is strictly READ-ONLY (the JSONL file, never br writes).
#
# Posture: WARN-ONLY with threshold 0 by default (informational baseline
# period — always prints the measured rate). Ratchet later with
# --strict --threshold N (or AGENTOPS_VERDICT_CLOSE_RATE_{STRICT,THRESHOLD}).
# Window: default 20 closes; --window N or AGENTOPS_VERDICT_CLOSE_RATE_WINDOW.
# SKIP=1 (AGENTOPS_VERDICT_CLOSE_RATE_SKIP=1) short-circuits with a clean skip.
#
# Exit codes: 0 = PASS, below-threshold WARN (non-strict), or clean SKIP;
#             1 = --strict and rate below threshold; 2 = bad invocation.
#
# Usage: bash scripts/check-verdict-close-rate.sh [--window N] [--threshold PCT] [--strict] [--json]

set -uo pipefail

THRESHOLD="${AGENTOPS_VERDICT_CLOSE_RATE_THRESHOLD:-0}"
WINDOW="${AGENTOPS_VERDICT_CLOSE_RATE_WINDOW:-20}"
STRICT="${AGENTOPS_VERDICT_CLOSE_RATE_STRICT:-0}"
JSON=0

while [ $# -gt 0 ]; do
  case "$1" in
    --threshold) shift; THRESHOLD="${1:?--threshold needs a value}" ;;
    --window)    shift; WINDOW="${1:?--window needs a value}" ;;
    --strict)    STRICT=1 ;;
    --json)      JSON=1 ;;
    -h|--help)   grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "check-verdict-close-rate: unknown arg: $1" >&2; exit 2 ;;
  esac
  shift
done

# Operator override
if [ "${AGENTOPS_VERDICT_CLOSE_RATE_SKIP:-0}" = "1" ]; then
  echo "check-verdict-close-rate: SKIP (AGENTOPS_VERDICT_CLOSE_RATE_SKIP=1)"
  exit 0
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "check-verdict-close-rate: SKIP (jq not on PATH)"
  exit 0
fi

# Resolve the br ledger dir: explicit $BEADS_DIR wins, else `ao beads dir`.
LEDGER_DIR="${BEADS_DIR:-}"
if [ -z "$LEDGER_DIR" ] && command -v ao >/dev/null 2>&1; then
  LEDGER_DIR="$(ao beads dir 2>/dev/null || true)"
fi
if [ -z "$LEDGER_DIR" ]; then
  echo "check-verdict-close-rate: SKIP (no BEADS_DIR and 'ao beads dir' unavailable — br ledger unresolvable here)"
  exit 0
fi

LEDGER="$LEDGER_DIR/issues.jsonl"
if [ ! -f "$LEDGER" ]; then
  echo "check-verdict-close-rate: SKIP (br ledger absent at $LEDGER)"
  exit 0
fi

# JSONL is last-wins per id: dedupe to the final record per bead, keep only
# closed ones, then take the newest N by closed_at.
STATS="$(jq -n -c --argjson n "$WINDOW" '
  [inputs]
  | reduce .[] as $i ({}; .[$i.id] = $i)
  | [.[]]
  | map(select(.status == "closed"))
  | sort_by(.closed_at // "") | reverse | .[0:$n]
  | {total: length,
     stamped: (map(select((.close_reason // "") | test("\\[verdict:"))) | length)}
' < "$LEDGER" 2>/dev/null)" || STATS=""
if [ -z "$STATS" ]; then
  echo "check-verdict-close-rate: SKIP (could not parse $LEDGER as JSONL)"
  exit 0
fi

TOTAL="$(printf '%s' "$STATS" | jq -r '.total')"
STAMPED="$(printf '%s' "$STATS" | jq -r '.stamped')"
if [ -z "$TOTAL" ] || [ "$TOTAL" = "null" ] || [ "$TOTAL" -eq 0 ] 2>/dev/null; then
  echo "check-verdict-close-rate: SKIP (no closed beads in window at $LEDGER)"
  exit 0
fi

RATE=$(( STAMPED * 100 / TOTAL ))

RESULT=PASS
if [ "$RATE" -lt "$THRESHOLD" ]; then
  if [ "$STRICT" = "1" ]; then RESULT=FAIL; else RESULT=WARN; fi
fi

if [ "$JSON" -eq 1 ]; then
  printf '{"stamped":%d,"total":%d,"rate_pct":%d,"threshold_pct":%d,"window":%d,"strict":%s,"result":"%s"}\n' \
    "$STAMPED" "$TOTAL" "$RATE" "$THRESHOLD" "$WINDOW" \
    "$([ "$STRICT" = "1" ] && echo true || echo false)" "$RESULT"
else
  case "$RESULT" in
    PASS)
      echo "check-verdict-close-rate: PASS — ${STAMPED}/${TOTAL} of last $WINDOW closes carry a [verdict:...] stamp = ${RATE}% (threshold ${THRESHOLD}%)"
      ;;
    WARN)
      echo "check-verdict-close-rate: WARN — ${STAMPED}/${TOTAL} of last $WINDOW closes carry a [verdict:...] stamp = ${RATE}% (< ${THRESHOLD}% threshold; warn-only)"
      echo "  fix: close beads via 'ao done <id>' so close_reason carries the verdict stamp"
      ;;
    FAIL)
      echo "check-verdict-close-rate: FAIL — ${STAMPED}/${TOTAL} of last $WINDOW closes carry a [verdict:...] stamp = ${RATE}% (< ${THRESHOLD}% threshold, --strict)"
      echo "  fix: close beads via 'ao done <id>' so close_reason carries the verdict stamp"
      ;;
  esac
fi

[ "$RESULT" = "FAIL" ] && exit 1
exit 0
