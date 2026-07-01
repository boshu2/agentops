#!/usr/bin/env bash
# check-provenance-feed-health.sh — assert the provenance sensor stays fed.
#
# Anti-regression gate: fails if the provenance ledger has stopped growing
# beyond the genesis row (the emitters silently died), or if the newest
# VERDICT edge (from_type=verdict, relation=wasDerivedFrom) is missing/stale
# (the verdict auto-binder silently died — age-wedge-all-in-dyr0.3). Keeps
# milestone-1 "fed" an enforced outcome, not a hope.
#
# Posture: WARN-ONLY by default (exit 0 with warnings). Pass --strict
# (or AGENTOPS_PROVENANCE_FEED_STRICT=1) to exit non-zero when the
# ledger has only the genesis row (dead sensor) or the newest verdict
# edge is absent/older than PROVENANCE_VERDICT_MAX_AGE_DAYS (default 14).
#
# Exit codes:
#   0 = ledger has real edges + recent verdict edge (healthy), or warn-only
#   1 = --strict and dead sensor / missing / stale verdict edge
#   2 = script error (missing ledger file, bad invocation)

set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
LEDGER="${PROVENANCE_LEDGER:-$REPO_ROOT/docs/provenance/ledger.jsonl}"
STRICT="${AGENTOPS_PROVENANCE_FEED_STRICT:-0}"

while [ $# -gt 0 ]; do
  case "$1" in
    --strict) STRICT=1 ;;
    -h|--help)
      sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      echo "check-provenance-feed-health: unknown argument: $1" >&2
      exit 2
      ;;
  esac
  shift
done

if [ ! -f "$LEDGER" ]; then
  echo "check-provenance-feed-health: FAIL — ledger missing at $LEDGER"
  exit 2
fi

# RFC3339 UTC ts ("2026-07-01T12:00:00Z") -> epoch seconds. GNU date first,
# then BSD date -j -f (macOS); prints nothing on failure (caller skips the
# recency math rather than fake a result). Same GNU-then-BSD guard pattern as
# the fleet portability sweep.
epoch_of() {
  date -u -d "$1" +%s 2>/dev/null && return 0
  date -u -j -f "%Y-%m-%dT%H:%M:%SZ" "$1" +%s 2>/dev/null
}

ROW_COUNT=0
NEWEST_VERDICT_TS=""
while IFS= read -r line; do
  [ -z "$line" ] && continue
  ROW_COUNT=$((ROW_COUNT + 1))
  # Newest VERDICT edge ts: rows with from_type=verdict AND
  # relation=wasDerivedFrom (the shape ao provenance emit-verdict writes).
  # RFC3339 UTC strings compare correctly as plain strings.
  case "$line" in *'"from_type":"verdict"'*) : ;; *) continue ;; esac
  case "$line" in *'"relation":"wasDerivedFrom"'*) : ;; *) continue ;; esac
  ts="$(printf '%s' "$line" | sed -n 's/.*"ts":"\([^"]*\)".*/\1/p')"
  [ -n "$ts" ] && [ "$ts" \> "$NEWEST_VERDICT_TS" ] && NEWEST_VERDICT_TS="$ts"
done < "$LEDGER"

FAILED=0

# --- landed-event growth (the original check, posture unchanged) -------------
if [ "$ROW_COUNT" -gt 1 ]; then
  echo "check-provenance-feed-health: PASS ($ROW_COUNT edges in ledger)"
elif [ "$STRICT" = "1" ]; then
  echo "check-provenance-feed-health: FAIL — ledger has only ${ROW_COUNT} row(s) (dead sensor)"
  echo "  path: $LEDGER"
  echo "  fix:  provenance emitters are not firing; check ao provenance emit-landed"
  FAILED=1
else
  echo "check-provenance-feed-health: WARN — ledger has only ${ROW_COUNT} row(s) (dead sensor)"
  echo "  path: $LEDGER"
fi

# --- verdict-event recency (age-wedge-all-in-dyr0.3) -------------------------
# A dead verdict auto-binder must be visible: warn (strict: fail) when the
# newest verdict edge is missing or older than the threshold.
VERDICT_MAX_AGE_DAYS="${PROVENANCE_VERDICT_MAX_AGE_DAYS:-14}"
if [ -z "$NEWEST_VERDICT_TS" ]; then
  if [ "$STRICT" = "1" ]; then
    echo "check-provenance-feed-health: FAIL — no verdict edges in ledger (verdict sensor never fired)"
    echo "  fix:  pawl-verdict.sh write/rebind emit via ao provenance emit-verdict; check the auto-binder"
    FAILED=1
  else
    echo "check-provenance-feed-health: WARN — no verdict edges in ledger (verdict sensor never fired)"
  fi
else
  NOW_EPOCH="$(date -u +%s)"
  VERDICT_EPOCH="$(epoch_of "$NEWEST_VERDICT_TS")" || VERDICT_EPOCH=""
  if [ -z "$VERDICT_EPOCH" ]; then
    echo "check-provenance-feed-health: WARN — cannot parse verdict ts '$NEWEST_VERDICT_TS' (skipping recency check)"
  else
    AGE_DAYS=$(( (NOW_EPOCH - VERDICT_EPOCH) / 86400 ))
    if [ "$AGE_DAYS" -gt "$VERDICT_MAX_AGE_DAYS" ]; then
      if [ "$STRICT" = "1" ]; then
        echo "check-provenance-feed-health: FAIL — newest verdict edge is ${AGE_DAYS}d old (max ${VERDICT_MAX_AGE_DAYS}d) — verdict auto-binder may be dead"
        echo "  newest: $NEWEST_VERDICT_TS"
        FAILED=1
      else
        echo "check-provenance-feed-health: WARN — newest verdict edge is ${AGE_DAYS}d old (max ${VERDICT_MAX_AGE_DAYS}d) — verdict auto-binder may be dead"
        echo "  newest: $NEWEST_VERDICT_TS"
      fi
    else
      echo "check-provenance-feed-health: PASS (newest verdict edge ${AGE_DAYS}d old, max ${VERDICT_MAX_AGE_DAYS}d)"
    fi
  fi
fi

[ "$FAILED" -eq 0 ] || exit 1
exit 0
