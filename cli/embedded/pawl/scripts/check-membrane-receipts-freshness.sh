#!/usr/bin/env bash
# check-membrane-receipts-freshness.sh — warn when the membrane receipts page
# has drifted stale behind the provenance ledger.
#
# The receipts page (docs/evidence/membrane-receipts.md) is generated from
# docs/provenance/ledger.jsonl by scripts/gen-membrane-receipts.sh. This check
# WARNS when the page's generated-at timestamp is more than 30 days older
# than the ledger tip entry's timestamp (or when the page is missing).
#
# Posture: WARN-ONLY by default (exit 0 with a warning). Pass --strict
# (or AGENTOPS_RECEIPTS_FRESHNESS_STRICT=1) to exit non-zero on staleness.
#
# Exit codes:
#   0 = receipts fresh, or warn-only with stale/missing receipts
#   1 = --strict and receipts are stale/missing
#   2 = script error (missing ledger, unparseable timestamps, bad invocation)

set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
LEDGER="${PROVENANCE_LEDGER:-$REPO_ROOT/docs/provenance/ledger.jsonl}"
RECEIPTS="${RECEIPTS_MD:-$REPO_ROOT/docs/evidence/membrane-receipts.md}"
STRICT="${AGENTOPS_RECEIPTS_FRESHNESS_STRICT:-0}"
MAX_AGE_DAYS=30

while [ $# -gt 0 ]; do
  case "$1" in
    --strict) STRICT=1 ;;
    -h|--help)
      sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      echo "check-membrane-receipts-freshness: unknown argument: $1" >&2
      exit 2
      ;;
  esac
  shift
done

fail_or_warn() {
  local msg="$1"
  if [ "$STRICT" = "1" ]; then
    echo "check-membrane-receipts-freshness: FAIL — $msg"
    echo "  fix: run scripts/gen-membrane-receipts.sh"
    exit 1
  fi
  echo "check-membrane-receipts-freshness: WARN — $msg"
  echo "  fix: run scripts/gen-membrane-receipts.sh"
  exit 0
}

# Portable ISO-8601 (UTC, ...Z) → epoch seconds: GNU date first, BSD fallback.
iso_to_epoch() {
  local ts="$1"
  if date -u -d "$ts" +%s 2>/dev/null; then
    return 0
  fi
  date -j -u -f "%Y-%m-%dT%H:%M:%SZ" "$ts" +%s 2>/dev/null
}

if [ ! -f "$LEDGER" ]; then
  echo "check-membrane-receipts-freshness: FAIL — ledger missing at $LEDGER" >&2
  exit 2
fi

TIP_TS="$(grep -o '"ts":"[^"]*"' "$LEDGER" | tail -1 | cut -d'"' -f4)"
if [ -z "$TIP_TS" ]; then
  echo "check-membrane-receipts-freshness: FAIL — could not read tip ts from $LEDGER" >&2
  exit 2
fi

if [ ! -f "$RECEIPTS" ]; then
  fail_or_warn "receipts page missing at $RECEIPTS (ledger tip: $TIP_TS)"
fi

GEN_TS="$(grep -m1 '^Generated: ' "$RECEIPTS" | cut -d' ' -f2)"
if [ -z "$GEN_TS" ]; then
  fail_or_warn "receipts page at $RECEIPTS has no 'Generated: <ts>' line"
fi

TIP_EPOCH="$(iso_to_epoch "$TIP_TS")" || TIP_EPOCH=""
GEN_EPOCH="$(iso_to_epoch "$GEN_TS")" || GEN_EPOCH=""
if [ -z "$TIP_EPOCH" ] || [ -z "$GEN_EPOCH" ]; then
  echo "check-membrane-receipts-freshness: FAIL — could not parse timestamps (tip=$TIP_TS gen=$GEN_TS)" >&2
  exit 2
fi

AGE_SECONDS=$((TIP_EPOCH - GEN_EPOCH))
MAX_AGE_SECONDS=$((MAX_AGE_DAYS * 86400))

if [ "$AGE_SECONDS" -gt "$MAX_AGE_SECONDS" ]; then
  fail_or_warn "receipts page is $((AGE_SECONDS / 86400)) day(s) older than the ledger tip (limit: ${MAX_AGE_DAYS}d; generated $GEN_TS, tip $TIP_TS)"
fi

echo "check-membrane-receipts-freshness: PASS (receipts generated $GEN_TS, ledger tip $TIP_TS)"
exit 0
