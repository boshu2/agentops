#!/usr/bin/env bash
# check-dream-throughput.sh — Gate: dream-cycle close-loop pipeline produced
# non-zero throughput for the candidates it ingested.
#
# Guards against the chicken-and-egg gate deadlock documented in
# .agents/learnings/2026-04-22-close-loop-citation-gate-deadlock.md: when
# fresh candidates were ingested but none reached auto-promotion, the pipeline
# is stalled regardless of how "clean" each stage looks in isolation.
#
# Consumes the JSON shape emitted by `ao flywheel close-loop --json`:
#   {"ingest":{"added":N},"auto_promote":{"promoted":M}}
#
# Exit codes:
#   0  PASS  — either no ingest this run, or promoted > 0
#   1  FAIL  — ingest.added > 0 && auto_promote.promoted == 0 (stall)
#   2  ERROR — missing/invalid input
#
# Usage: check-dream-throughput.sh <close-loop.json>
set -euo pipefail

INPUT="${1:-}"
if [[ -z "$INPUT" ]]; then
  echo "usage: $0 <close-loop.json>" >&2
  exit 2
fi
if [[ ! -f "$INPUT" ]]; then
  echo "ERROR: $INPUT not found" >&2
  exit 2
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "ERROR: jq not found" >&2
  exit 2
fi

added="$(jq -r '.ingest.added // 0' "$INPUT" 2>/dev/null || echo "")"
promoted="$(jq -r '.auto_promote.promoted // 0' "$INPUT" 2>/dev/null || echo "")"

if ! [[ "$added" =~ ^[0-9]+$ ]] || ! [[ "$promoted" =~ ^[0-9]+$ ]]; then
  echo "ERROR: could not parse .ingest.added / .auto_promote.promoted as integers in $INPUT" >&2
  exit 2
fi

if (( added > 0 && promoted == 0 )); then
  cat >&2 <<EOF
FAIL: dream close-loop throughput stall
      ingest.added=$added but auto_promote.promoted=0
      A nonzero ingest with zero promotions is a deadlocked pipeline, not a
      steady state. See .agents/learnings/2026-04-22-close-loop-citation-gate-deadlock.md.
EOF
  exit 1
fi

if (( added == 0 )); then
  echo "OK: no candidates ingested this run (ingest.added=0); nothing to promote"
  exit 0
fi

echo "PASS: auto_promote.promoted=$promoted / ingest.added=$added"
exit 0
