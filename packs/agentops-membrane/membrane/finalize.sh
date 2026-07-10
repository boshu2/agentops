#!/usr/bin/env bash
# finalize.sh — pure, side-effect-free deterministic close-door decision.
#
# Given the review lane JSONs (review-quorum.lane.v1 + our agentops_nonce echo),
# an expected per-round nonce, and the expected cross-family posture, it emits a
# canonical pawl-verdict.v1 only for semantic terminal results. Transport
# degradation emits gc-review-attempt.v1 instead. It performs NO
# slinging, NO waiting, NO bead writes, NO network — that is the gate's job
# (close-gate.sh). Keeping the decision here, isolated and dependency-light
# (bash + jq only), is what makes the membrane's verdict cheaply unit-testable
# (tests/finalize.bats) without a live drill.
#
# Exit codes (consumed by close-gate.sh; NOT the [steps.check] contract):
#   0 = CONFIRMED  (all lanes pass, >=2 families, nonces match) -> close
#   2 = REFUTED    (a hard finding / contract / nonce violation) -> respawn builder, consume attempt
#   3 = DEGRADED   (transient lane loss / awaiting) -> retry; native GC still
#                   consumes the failed check attempt
#
# Usage:
#   finalize.sh --nonce N --round R --subject BEAD --base-ref REF \
#     --expected-families "gpt,gemini" --head-sha SHA --author AUTHOR \
#     --out /path/pawl-verdict.json --attempt-out /path/review-attempt.json \
#     [--evidence-dir DIR] LANE1.json [LANE2.json ...]
set -u

NONCE=""; ROUND="1"; SUBJECT=""; BASE_REF="origin/main"
EXPECTED_FAMILIES=""; HEAD_SHA=""; AUTHOR=""; OUT=""; ATTEMPT_OUT=""; EVDIR=""
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
    --attempt-out) ATTEMPT_OUT="$2"; shift 2 ;;
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
[ -z "$ATTEMPT_OUT" ] && ATTEMPT_OUT="${OUT%.json}.attempt.json"
[ -z "$EVDIR" ] && EVDIR="$(dirname "$OUT")/evidence"

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

# --- persist exactly one artifact class ------------------------------------
TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
if [ "$DISPOSITION" = "DEGRADED" ]; then
  rm -f "$OUT"
  mkdir -p "$(dirname "$ATTEMPT_OUT")"
  printf '%s' "$DECISION" | jq \
    --arg schema "gc-review-attempt.v1" \
    --arg outcome "$DISPOSITION" \
    --arg ts "$TS" \
    --arg sha "$HEAD_SHA" \
    '{schema_version:$schema,outcome:$outcome,failure_class:.failure_class,
      failure_reason:.failure_reason,subject:.subject,head_sha:$sha,
      generated_at:$ts,round:.round,nonce:.nonce,
      expected_families:.expected_families,present_families:.present_families}' > "$ATTEMPT_OUT"
  echo "$DISPOSITION $FAILURE_CLASS $FAILURE_REASON"
  exit 3
fi

rm -f "$ATTEMPT_OUT"
mkdir -p "$(dirname "$OUT")" "$EVDIR"
EVIDENCE_PATHS='[]'
idx=0
for lane_file in "${LANES[@]:-}"; do
  [ -n "$lane_file" ] && [ -s "$lane_file" ] && jq -e '.' "$lane_file" >/dev/null 2>&1 || continue
  idx=$((idx + 1))
  evidence_path="$EVDIR/lane-$idx.json"
  cp "$lane_file" "$evidence_path"
  EVIDENCE_PATHS="$(printf '%s' "$EVIDENCE_PATHS" | jq --arg path "$evidence_path" '. + [$path]')"
done

printf '%s' "$LANE_ARR" | jq \
  --arg schema "pawl-verdict.v1" \
  --arg bead "$SUBJECT" \
  --arg sha "$HEAD_SHA" \
  --arg disp "$DISPOSITION" \
  --arg ts "$TS" \
  --arg author "$AUTHOR" \
  --arg nonce "$NONCE" \
  --arg round "$ROUND" \
  --argjson evidence_paths "$EVIDENCE_PATHS" \
  'def norm: (. // "") | tostring | ascii_downcase;
   def lane_refuted:
     ((.verdict | norm) == "fail") or ((.verdict | norm) == "blocked") or
     ((.verdict | norm) != "pass" and (.verdict | norm) != "pass_with_findings") or
     (((.agentops_nonce // "") | tostring) != $nonce) or
     ((.failure_class | norm) == "hard") or
     ((.read_only_enforcement.passed // false) != true) or
     (((.mutations_delta.changed // []) | length) > 0);
   (map(lane_refuted) | any) as $has_lane_refutation |
   {
     schema_version: $schema, bead_id: $bead, pr: 0, head_sha: $sha,
     disposition: $disp, generated_at: $ts, mode: "multi-model",
     author_context_id: $author, attempt: ($round|tonumber? // 1),
     refuters: [ to_entries[] | .key as $idx | .value as $lane | {
        family: ($lane.provider // ""), reviewer: ($lane.lane_id // ""),
        context_id: ($lane.lane_id // ""),
        verdict: (if $disp == "CONFIRMED" then "CONFIRMED"
                  elif ($lane | lane_refuted) or (($has_lane_refutation | not) and $idx == 0)
                  then "REFUTED" else "CONFIRMED" end),
        evidence: $evidence_paths[$idx]
     } ]
   }' > "$OUT"

echo "$DISPOSITION $FAILURE_CLASS $FAILURE_REASON"

case "$DISPOSITION" in
  CONFIRMED) exit 0 ;;
  REFUTED)   exit 2 ;;
  *)         exit 2 ;;
esac
