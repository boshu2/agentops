#!/usr/bin/env bash
# finalize.sh — pure, side-effect-free deterministic close-door decision.
#
# Given the review lane JSONs (review-quorum.lane.v1 + our agentops_nonce echo),
# an expected per-round nonce, and the expected cross-family posture, it emits a
# pawl-verdict.v1 artifact and returns a disposition exit code. It performs NO
# slinging, NO waiting, NO bead writes, NO network — that is the gate's job
# (close-gate.sh). Keeping the decision here, isolated and dependency-light
# (bash + jq only), is what makes the membrane's verdict cheaply unit-testable
# (tests/finalize.bats) without a live drill.
#
# Exit codes (consumed by close-gate.sh; NOT the [steps.check] contract):
#   0 = CONFIRMED  (all lanes pass, >=2 families, nonces match) -> close
#   2 = REFUTED    (a hard finding / contract / nonce violation) -> respawn builder, consume attempt
#   3 = DEGRADED   (transient lane loss / awaiting) -> retry lane, DO NOT consume attempt
#
# Usage:
#   finalize.sh --nonce N --round R --subject BEAD --base-ref REF \
#     --expected-families "gpt,gemini" --head-sha SHA --author AUTHOR \
#     --out /path/pawl-verdict.json [--evidence-dir DIR] LANE1.json [LANE2.json ...]
set -u

NONCE=""; ROUND="1"; SUBJECT=""; BASE_REF="origin/main"
EXPECTED_FAMILIES=""; HEAD_SHA=""; AUTHOR=""; OUT=""; EVDIR=""
LANES=()
while [ $# -gt 0 ]; do
  case "$1" in
    --nonce) NONCE="$2"; shift 2 ;;
    --round) ROUND="$2"; shift 2 ;;
    --subject) SUBJECT="$2"; shift 2 ;;
    --base-ref) BASE_REF="$2"; shift 2 ;;
    --expected-families) EXPECTED_FAMILIES="$2"; shift 2 ;;
    --head-sha) HEAD_SHA="$2"; shift 2 ;;
    --author) AUTHOR="$2"; shift 2 ;;
    --out) OUT="$2"; shift 2 ;;
    --evidence-dir) EVDIR="$2"; shift 2 ;;
    --) shift; while [ $# -gt 0 ]; do LANES+=("$1"); shift; done ;;
    *) LANES+=("$1"); shift ;;
  esac
done

if ! command -v jq >/dev/null 2>&1; then
  echo "finalize: jq not found" >&2; exit 2
fi
if [ -z "$OUT" ]; then echo "finalize: --out is required" >&2; exit 2; fi
if [ -z "$NONCE" ]; then echo "finalize: --nonce is required" >&2; exit 2; fi

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FINALIZE_JQ="$HERE/finalize.jq"

# --- collect the present, parseable lane JSONs into one array --------------
LANE_ARR="$(
  { for f in "${LANES[@]:-}"; do
      [ -n "$f" ] && [ -s "$f" ] && jq -c '.' "$f" 2>/dev/null
    done; } | jq -s '.'
)"
[ -z "$LANE_ARR" ] && LANE_ARR="[]"

# --- deterministic decision (finalize.jq) ----------------------------------
# The program reads all data from jq variables (--argjson lanes / --arg ...);
# `-n` makes stdin (`.`) irrelevant. Portable, no jq modules.
DECISION="$(
  jq -n \
    --argjson lanes "$LANE_ARR" \
    --arg nonce "$NONCE" \
    --arg round "$ROUND" \
    --arg subject "$SUBJECT" \
    --arg base_ref "$BASE_REF" \
    --arg expected_families "$EXPECTED_FAMILIES" \
    -f "$FINALIZE_JQ" 2>/dev/null
)"

if [ -z "$DECISION" ]; then
  echo "finalize: decision computation failed" >&2
  DECISION='{"disposition":"DEGRADED","failure_class":"transient","failure_reason":"finalize_internal_error","findings_count":0}'
fi

DISPOSITION="$(printf '%s' "$DECISION" | jq -r '.disposition')"
FAILURE_CLASS="$(printf '%s' "$DECISION" | jq -r '.failure_class')"
FAILURE_REASON="$(printf '%s' "$DECISION" | jq -r '.failure_reason')"

# --- persist the pawl-verdict.v1 artifact ----------------------------------
TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
mkdir -p "$(dirname "$OUT")"
printf '%s' "$LANE_ARR" | jq \
  --arg schema "pawl-verdict.v1" \
  --arg bead "$SUBJECT" \
  --arg sha "$HEAD_SHA" \
  --arg disp "$DISPOSITION" \
  --arg fclass "$FAILURE_CLASS" \
  --arg freason "$FAILURE_REASON" \
  --arg ts "$TS" \
  --arg author "$AUTHOR" \
  --arg nonce "$NONCE" \
  --arg round "$ROUND" \
  --arg evdir "$EVDIR" \
  '{
     schema_version: $schema, bead_id: $bead, pr: 0, head_sha: $sha,
     disposition: $disp, failure_class: $fclass, failure_reason: $freason,
     generated_at: $ts, author_context_id: $author,
     nonce: $nonce, round: ($round|tonumber? // 0),
     refuters: [ .[] | {
        family: (.provider // ""), verdict: (.verdict // ""),
        lane_id: (.lane_id // ""), context_id: (.lane_id // ""),
        nonce_echo: ((.agentops_nonce // "")|tostring),
        findings_count: (.findings_count // 0),
        evidence: (if $evdir != "" then ($evdir + "/" + (.lane_id // "lane") + ".json") else "" end)
     } ]
   }' > "$OUT"

echo "$DISPOSITION $FAILURE_CLASS $FAILURE_REASON"

case "$DISPOSITION" in
  CONFIRMED) exit 0 ;;
  REFUTED)   exit 2 ;;
  DEGRADED)  exit 3 ;;
  *)         exit 2 ;;
esac
