#!/usr/bin/env bash
# close-gate.sh — the AgentOps membrane's FAIL-CLOSED, VERDICT-BOUND close door.
#
# This is the one structural thing stock Gas City lacks (out-of-box gap map,
# BOTTOM LINE §2): stock gc ships mol-review-quorum's cross-family fan-out but
# its verdict is agent-written and its Go finalizer (reviewquorum.Finalize) has
# ZERO production callers, so an agent can synthesize "pass" over failing lanes
# and nothing stops the close. This door replaces that with a DETERMINISTIC,
# fail-closed verdict: no valid CONFIRMED cross-family verdict => the source
# bead STAYS OPEN.
#
# Runs as the [steps.check] exec of formulas/membrane-quest.toml, invoked by the
# core control-dispatcher (never by an agent) after each build iteration closes.
# It:
#   1. deterministic pre-gates (windshield: branch exists, diff non-empty,
#      contract present) BEFORE burning any reviewer tokens;
#   2. mints a per-round NONCE and routes ONLY the diff + acceptance contract to
#      >=2 CROSS-FAMILY, FRESH-CONTEXT reviewer lanes (LAW 0: never claude -p);
#   3. collects each lane's review-quorum.lane.v1 JSON (+ nonce echo) and hands
#      them to membrane/finalize.sh — the DETERMINISTIC verdict (a faithful port
#      of reviewquorum.Finalize; see finalize.jq header for why a port);
#   4. fail-closes: CONFIRMED (exit 0) closes; a hard finding REFUTES (respawn
#      builder, consume an attempt); transient lane loss DEGRADES into a
#      nonsemantic attempt artifact — degradation is never a false refute, but
#      native GC consumes every failed check attempt.
#
# It NEVER merges, pushes, or touches any branch. A human merges.
#
# Env from the dispatcher (internal/convergence/condition.go):
#   GC_BEAD_ID (iteration bead), GC_ITERATION, GC_MAX_ITERATIONS, GC_CITY_PATH.
# Overridable knobs (city may set; defaults assume [imports.agentops-membrane]):
#   MEMBRANE_LANE1_TARGET / MEMBRANE_LANE1_FAMILY   (default agentops-membrane.verifier / gpt)
#   MEMBRANE_LANE2_TARGET / MEMBRANE_LANE2_FAMILY   (default agentops-membrane.agy-verifier / gemini)
#   MEMBRANE_HELPER_TARGET (default agentops-membrane.breaker-helper)
#   MEMBRANE_QUEST_ROOT (default $GC_CITY_PATH/quests)   MEMBRANE_WAIT_SECS (default 300)
set -u

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FINALIZE="$HERE/finalize.sh"

CITY="${GC_CITY_PATH:-$PWD}"
GC_BIN="${GC_BIN:-$(command -v gc || echo gc)}"
ROUND="${GC_ITERATION:-1}"
ENV_MAXR="${GC_MAX_ITERATIONS:-}"
WAIT_SECS="${MEMBRANE_WAIT_SECS:-300}"
HELPER_WAIT_SECS="${MEMBRANE_HELPER_WAIT_SECS:-120}"
GATE_TIMEOUT_SECS="${MEMBRANE_GATE_TIMEOUT_SECS:-480}"
GATE_SAFETY_SECS="${MEMBRANE_GATE_SAFETY_SECS:-30}"
QUEST_ROOT="${MEMBRANE_QUEST_ROOT:-$CITY/quests}"

# The helper runs after the reviewer wait on the breaker round. Keep their
# combined waits strictly inside the formula's 8m check timeout so GC cannot
# kill the process before it persists the helper outcome/fallback. Operator
# overrides are clamped, never trusted to violate the enclosing lease.
case "$WAIT_SECS:$HELPER_WAIT_SECS:$GATE_TIMEOUT_SECS:$GATE_SAFETY_SECS" in
  *[!0-9:]*|*::*|:*|*:) echo "[membrane-gate] invalid wait-budget value" >&2; exit 1 ;;
esac
MAX_COMBINED_WAIT=$((GATE_TIMEOUT_SECS - GATE_SAFETY_SECS))
[ "$MAX_COMBINED_WAIT" -lt 0 ] && MAX_COMBINED_WAIT=0
[ "$WAIT_SECS" -gt "$MAX_COMBINED_WAIT" ] && WAIT_SECS="$MAX_COMBINED_WAIT"
HELPER_WAIT_BUDGET=$((MAX_COMBINED_WAIT - WAIT_SECS))
[ "$HELPER_WAIT_SECS" -gt "$HELPER_WAIT_BUDGET" ] && HELPER_WAIT_SECS="$HELPER_WAIT_BUDGET"

LANE1_TARGET="${MEMBRANE_LANE1_TARGET:-agentops-membrane.verifier}"
LANE1_FAMILY="${MEMBRANE_LANE1_FAMILY:-gpt}"
LANE2_TARGET="${MEMBRANE_LANE2_TARGET:-agentops-membrane.agy-verifier}"
LANE2_FAMILY="${MEMBRANE_LANE2_FAMILY:-gemini}"
HELPER_TARGET="${MEMBRANE_HELPER_TARGET:-agentops-membrane.breaker-helper}"
HELPER_ROUND=-1
HELPER_ROUTED=0

log() { echo "[membrane-gate r$ROUND] $*" >&2; }
bdq() { (cd "$CITY" && bd "$@"); }
bead_json() { bdq show "$1" --json 2>/dev/null | jq 'if type=="array" then .[0] else . end'; }

# --- stamp the iteration bead's failure class then exit non-zero (retry) -----
# HONEST CONTRACT (corrected 2026-07-06 RCA): on native graph.v2 the ralph
# dispatcher NEVER reads gc.failure_class — EVERY nonzero check exit consumes
# an attempt (transient and hard alike; gc internal/dispatch/control.go). The
# class stamp below is evidence/telemetry for humans and the finalizer's
# artifact trail, not retry-budget control. Budget transient flakes via the
# formula's max_attempts instead.
retry_exit() {  # $1=class(transient|hard) $2=reason
  if [ "$ROUND" -eq "$HELPER_ROUND" ] && [ "$HELPER_ROUTED" -eq 0 ] && declare -F route_breaker_helper >/dev/null; then
    HELPER_ROUTED=1
    route_breaker_helper "$1" "$2"
  fi
  bdq update "$GC_BEAD_ID" \
    --set-metadata "gc.outcome=fail" \
    --set-metadata "gc.failure_class=$1" \
    --set-metadata "gc.failure_reason=$2" >/dev/null 2>&1 || true
  log "NOT CONFIRMED: failure_class=$1 reason=$2 -> step fails, dispatcher decides retry"
  exit 1
}

# --- resolve quest metadata from the iteration bead (native: via bd) ---------
ITERATION_JSON="$(bead_json "$GC_BEAD_ID")"
QUEST="$(printf '%s' "$ITERATION_JSON" | jq -r '.metadata.quest // empty')"
if [ -z "$QUEST" ]; then
  log "FAIL-CLOSED: iteration bead $GC_BEAD_ID carries no quest metadata"
  retry_exit hard gate_no_quest_metadata
fi
RUN_ID="$(printf '%s' "$ITERATION_JSON" | jq -r '.metadata["gc.root_bead_id"] // empty')"
case "$RUN_ID" in
  ""|*[!A-Za-z0-9._-]*)
    log "FAIL-CLOSED: iteration bead $GC_BEAD_ID carries no path-safe gc.root_bead_id"
    retry_exit hard gate_no_run_metadata ;;
esac
STEP_REF="$(printf '%s' "$ITERATION_JSON" | jq -r '.metadata["gc.step_ref"] // empty')"
# Attempt beads use <logical-ref>.iteration.N while the ralph control retains
# <logical-ref>. Normalize only that terminal attempt suffix before matching.
CONTROL_STEP_REF="$(printf '%s' "$STEP_REF" | sed -E 's/\.iteration\.[0-9]+$//')"
# graph.v2 currently leaves ConditionEnv.MaxIterations unset, which exports
# GC_MAX_ITERATIONS=0. Recover the authoritative budget from the open ralph
# control bead stamped by formula compilation. A positive env value is only a
# fallback; two positive sources must agree.
CONTROL_MAXR="$(bdq list --status open --include-gates --limit 0 --json 2>/dev/null | jq -r \
  --arg root "$RUN_ID" --arg ref "$CONTROL_STEP_REF" '
  [.[] | select(.metadata["gc.root_bead_id"] == $root)
       | select(.metadata["gc.kind"] == "ralph")
       | select($ref == "" or .metadata["gc.step_ref"] == $ref)
       | .metadata["gc.max_attempts"]
       | select(type == "string" and test("^[1-9][0-9]*$"))]
  | unique | if length == 1 then .[0] else empty end' 2>/dev/null)"
case "$ENV_MAXR" in ""|*[!0-9]*|0) ENV_MAXR="" ;; esac
if [ -n "$CONTROL_MAXR" ] && [ -n "$ENV_MAXR" ] && [ "$CONTROL_MAXR" != "$ENV_MAXR" ]; then
  log "FAIL-CLOSED: attempt budget mismatch control=$CONTROL_MAXR env=$ENV_MAXR"
  retry_exit hard gate_attempt_budget_mismatch
fi
MAXR="${CONTROL_MAXR:-$ENV_MAXR}"
if [ -z "$MAXR" ]; then
  log "FAIL-CLOSED: no authoritative positive max-attempts budget"
  retry_exit hard gate_no_attempt_budget
fi
HELPER_ROUND=$((MAXR - 1))
REPO="$QUEST_ROOT/$QUEST"
BRANCH="quest/$QUEST"
# Every sling gets a new workflow root. Scope all nonces, reviews, and helper
# receipts to that root so re-slinging the same quest cannot replay a prior
# run's verdict or breaker decision.
EX="$CITY/membrane/$QUEST/runs/$RUN_ID"
mkdir -p "$EX"

submit_target() {  # $1=target $2=request-text
  "$GC_BIN" --city "$CITY" session submit "$1" "$2" 2>&1
}

ACTIVE_HELPER_SESSION=""
ACTIVE_HELPER_RECEIPT=""
cleanup_helper_session() {
  [ -n "$ACTIVE_HELPER_SESSION" ] || return 0
  local session_id="$ACTIVE_HELPER_SESSION" receipt="$ACTIVE_HELPER_RECEIPT"
  # Clear first so signals/EXIT cannot dispatch a second close for the same
  # bounded consultation when the first close attempt fails.
  ACTIVE_HELPER_SESSION=""
  ACTIVE_HELPER_RECEIPT=""
  if ! "$GC_BIN" --city "$CITY" session close "$session_id" >/dev/null 2>&1; then
    return 1
  fi
  [ -n "$receipt" ] && printf '%s\n' "$session_id" > "$receipt.closed"
}
trap 'cleanup_helper_session || true' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

cumulative_breaker_evidence() {
  local file
  for file in \
    "$EX"/pawl-verdict-round-*.json \
    "$EX"/evidence-round-*/*.json \
    "$EX"/review-attempt-round-*.json; do
    [ -s "$file" ] || continue
    printf '\n--- %s ---\n' "$(basename "$file")"
    cat "$file"
  done
}

build_helper_request() {  # $1=outcome-path $2=nonce $3=class $4=reason
  local out="$1" helper_nonce="$2" failure_class="$3" failure_reason="$4"
  cat <<EOF
BREAKER HELPER — quest '$QUEST', after round $ROUND of $MAXR.

You are the ONE bounded helper consultation after the work circuit breaker
entered HOLD. You are an advisor, never the author or final verifier. Do not
read or mutate the repo and do not dispatch another helper. Decide only whether
the cumulative evidence contains one concrete, materially new approach.

Write exactly one JSON object to this path (your only write):
  $out
Schema:
  schema_version="agentops-breaker-helper.v1"
  outcome="UNSTUCK" when you can name a concrete new approach, otherwise "ESCALATE"
  new_approach=<nonempty string for UNSTUCK, empty string for ESCALATE>
  reason=<brief evidence-bound reason>
  evidence=<nonempty array of artifact/field references>
  agentops_nonce="$helper_nonce"

Current failure: class=$failure_class reason=$failure_reason

CUMULATIVE ATTEMPT EVIDENCE:
$(cumulative_breaker_evidence)
EOF
}

write_helper_escalate_fallback() {  # $1=out $2=nonce $3=reason
  jq -n --arg nonce "$2" --arg reason "$3" '{
    schema_version:"agentops-breaker-helper.v1",
    outcome:"ESCALATE",new_approach:"",reason:$reason,
    evidence:["helper transport or output validation failed"],agentops_nonce:$nonce
  }' > "$1"
}

valid_helper_outcome() {  # $1=path $2=nonce
  jq -e --arg nonce "$2" '
    .schema_version == "agentops-breaker-helper.v1" and
    (.outcome == "UNSTUCK" or .outcome == "ESCALATE") and
    (.reason | type == "string" and length > 0) and
    (.evidence | type == "array" and length > 0) and
    .agentops_nonce == $nonce and
    (if .outcome == "UNSTUCK" then (.new_approach | type == "string" and length > 0)
     else (.new_approach == "") end)
  ' "$1" >/dev/null 2>&1
}

route_breaker_helper() {  # $1=failure-class $2=failure-reason
  local failure_class="$1" failure_reason="$2"
  local out="$EX/breaker-helper-round-$ROUND.json"
  local nonce_file="$EX/breaker-helper-nonce-round-$ROUND.txt"
  local session_receipt="$EX/breaker-helper-session-round-$ROUND.json"
  local helper_nonce helper_alias helper_session_id create_json list_json waited=0
  if [ -s "$nonce_file" ]; then
    helper_nonce="$(cat "$nonce_file")"
  else
    helper_nonce="$(openssl rand -hex 8 2>/dev/null || echo "helper-r${ROUND}-$$")"
    printf '%s\n' "$helper_nonce" > "$nonce_file"
  fi
  if valid_helper_outcome "$out" "$helper_nonce"; then
    # Recover cleanup after a crash between the helper write and session close.
    # A successful close marker makes subsequent re-entry a pure artifact read.
    if [ -s "$session_receipt" ] && [ ! -s "$session_receipt.closed" ]; then
      ACTIVE_HELPER_SESSION="$(jq -r '.session_id // empty' "$session_receipt" 2>/dev/null)"
      ACTIVE_HELPER_RECEIPT="$session_receipt"
      if ! cleanup_helper_session; then
        write_helper_escalate_fallback "$out" "$helper_nonce" helper_session_close_failed
      fi
    fi
    cp "$out" "$EX/breaker-helper.json"
    log "helper outcome=$(jq -r '.outcome' "$out") artifact=$out (idempotent reuse)"
    return
  fi
  rm -f "$out"
  helper_alias="breaker-${QUEST}-r${ROUND}-${helper_nonce%????????}"
  log "CIRCUIT-BREAKER-TRIP -> HOLD -> ONE-HELPER fresh_session_template=$HELPER_TARGET"
  if [ -s "$session_receipt" ]; then
    helper_session_id="$(jq -r '.session_id // empty' "$session_receipt" 2>/dev/null)"
  else
    create_json="$("$GC_BIN" --city "$CITY" session new "$HELPER_TARGET" \
      --alias "$helper_alias" \
      --no-attach --json 2>/dev/null || true)"
    helper_session_id="$(printf '%s' "$create_json" | jq -r 'select(.ok == true) | .session_id // empty' 2>/dev/null)"
    # A crash after Gas City created the deterministic alias but before this
    # script persisted its receipt must recover that same session, never create
    # a second helper context.
    if [ -z "$helper_session_id" ]; then
      list_json="$("$GC_BIN" --city "$CITY" session list --json 2>/dev/null || true)"
      helper_session_id="$(printf '%s' "$list_json" | jq -r --arg alias "$helper_alias" \
        '[.sessions[]? | select(.alias == $alias and .closed != true)][0].id // empty' 2>/dev/null)"
      [ -n "$helper_session_id" ] && create_json="$(jq -n --arg id "$helper_session_id" --arg alias "$helper_alias" --arg template "$HELPER_TARGET" \
        '{ok:true,session_id:$id,session_name:"",alias:$alias,template:$template}')"
    fi
    if [ -n "$helper_session_id" ]; then
      printf '%s' "$create_json" | jq -c '{schema_version:"agentops-breaker-helper-session.v1",session_id,session_name,alias,template}' > "$session_receipt"
    fi
  fi
  if [ -z "$helper_session_id" ]; then
    write_helper_escalate_fallback "$out" "$helper_nonce" helper_fresh_session_create_failed
    cp "$out" "$EX/breaker-helper.json"
    log "helper outcome=ESCALATE reason=fresh_session_create_failed artifact=$out"
    return
  fi
  ACTIVE_HELPER_SESSION="$helper_session_id"
  ACTIVE_HELPER_RECEIPT="$session_receipt"
  # A persisted receipt with no outcome means a prior invocation already
  # submitted this one bounded consultation. Wait for it; never create or
  # submit a second helper for the same breaker round.
  if [ ! -s "$session_receipt.submitted" ]; then
    # Persist before submit: a crash can conservatively ESCALATE a consultation
    # that may not have launched, but can never dispatch it twice.
    printf '%s\n' "$helper_session_id" > "$session_receipt.submitted"
    submit_target "$helper_session_id" "$(build_helper_request "$out" "$helper_nonce" "$failure_class" "$failure_reason")" >/dev/null 2>&1 || true
  fi
  while [ "$waited" -lt "$HELPER_WAIT_SECS" ]; do
    [ -s "$out" ] && break
    sleep 5
    waited=$((waited+5))
  done
  if ! valid_helper_outcome "$out" "$helper_nonce"; then
    write_helper_escalate_fallback "$out" "$helper_nonce" helper_unavailable_or_invalid
  fi
  cp "$out" "$EX/breaker-helper.json"
  log "helper outcome=$(jq -r '.outcome' "$out") artifact=$out"
  if ! cleanup_helper_session; then
    write_helper_escalate_fallback "$out" "$helper_nonce" helper_session_close_failed
    cp "$out" "$EX/breaker-helper.json"
    log "helper outcome=ESCALATE reason=session_close_failed artifact=$out"
  fi
}

# Attempt MAXR is reserved for proving an UNSTUCK approach. ESCALATE (including
# helper transport failure) terminates without another review or mutation.
if [ "$ROUND" -eq "$MAXR" ]; then
  HELPER_OUT="$EX/breaker-helper-round-$HELPER_ROUND.json"
  HELPER_NONCE_FILE="$EX/breaker-helper-nonce-round-$HELPER_ROUND.txt"
  HELPER_NONCE="$(cat "$HELPER_NONCE_FILE" 2>/dev/null || true)"
  if [ -z "$HELPER_NONCE" ] || ! valid_helper_outcome "$HELPER_OUT" "$HELPER_NONCE"; then
    log "HOLD: recovery attempt has no valid helper outcome"
    retry_exit hard helper_outcome_missing
  fi
  case "$(jq -r '.outcome' "$HELPER_OUT")" in
    ESCALATE)
      log "HELPER-ESCALATE -> HUMAN (no recovery review dispatched)"
      retry_exit hard helper_escalate ;;
    UNSTUCK)
      log "HELPER-UNSTUCK -> AUTO-REDO; proving the new approach" ;;
    *)
      retry_exit hard helper_outcome_invalid ;;
  esac
fi

# --- per-round nonce (anti-replay / anti-stale-verdict) ----------------------
NONCE="$(openssl rand -hex 8 2>/dev/null || (date +%s%N; echo $RANDOM) | md5sum 2>/dev/null | cut -c1-16 || echo "r${ROUND}$$")"
echo "$NONCE" > "$EX/nonce-round-$ROUND.txt"

# --- deterministic pre-gates (windshield before verifier tokens) -------------
if ! git -C "$REPO" rev-parse --verify -q "$BRANCH" >/dev/null 2>&1; then
  log "pre-gate REFUTED: no quest branch $BRANCH (builder produced no reviewable commit)"
  retry_exit hard gate_no_branch
fi
HEAD_SHA="$(git -C "$REPO" rev-parse "$BRANCH" 2>/dev/null)"
DIFF="$(git -C "$REPO" diff "main...$BRANCH" 2>/dev/null)"
if [ -z "$DIFF" ]; then
  log "pre-gate REFUTED: quest branch has no diff against main (nothing to review)"
  retry_exit hard gate_empty_diff
fi
# the ruler is read from MAIN, never the builder's branch
CONTRACT="$(git -C "$REPO" show main:CONTRACT.md 2>/dev/null || true)"
if [ -z "$CONTRACT" ]; then
  log "FAIL-CLOSED: no CONTRACT.md on main of $REPO (no ruler to judge against)"
  retry_exit hard gate_no_contract
fi

# --- build the fresh-context review request (diff + contract + nonce ONLY) ---
build_request() {  # $1=lane_id $2=provider $3=out_json_path
  local lane_id="$1" provider="$2" out="$3"
  cat <<EOF
MEMBRANE VERIFICATION — quest '$QUEST' round $ROUND of $MAXR. Lane: $lane_id ($provider).

You are ONE lane in a cross-family review quorum and you are the JUDGE, not the
author. Your ENTIRE input is the acceptance contract and the diff below. Do NOT
read the repo, run code, or contact the builder. Default-FAIL: anything the diff
does not demonstrably satisfy is a finding.

PATH FRAME (read before raising any placement finding): the diff paths are
relative to the QUEST REPO ROOT — which the city mounts at quests/$QUEST/. A
contract reference to quests/$QUEST/<file> and a bare diff path <file> are the
SAME file in different frames. Never raise a file-placement/scope finding for
that frame difference alone; placement findings require a path that is wrong in
BOTH frames.

Write your durable verdict as review-quorum.lane.v1 JSON to EXACTLY this path
(the one file you may write):
  $out
It MUST include these keys:
  lane_id="$lane_id", provider="$provider", model=<your model>,
  verdict=one of pass|pass_with_findings|fail|blocked,
  summary, findings_count, findings[], evidence[], usage,
  read_only_enforcement{observed,enabled,passed,baseline_command,after_command},
  mutations_delta{changed:[]}, failure_class=none|transient|hard, failure_reason,
  AND — critical anti-replay — agentops_nonce="$NONCE" (echo it verbatim; a
  verdict without this exact nonce is rejected as stale).
If your provider is unavailable/rate-limited/timed-out, set verdict=blocked,
failure_class=transient, failure_reason=<provider_unavailable|provider_timeout|
rate_limited>. Use failure_class=hard only for a real contract failure.
End your reply with the sentinel line: VERDICT: CONFIRMED|REFUTED|BLOCKED ...

--- ACCEPTANCE CONTRACT (quests/$QUEST/CONTRACT.md @ main) ---
$CONTRACT

--- DIFF (git diff main...$BRANCH @ $HEAD_SHA) ---
\`\`\`diff
$DIFF
\`\`\`
EOF
}

LANE1_OUT="$EX/lane-${LANE1_FAMILY}-round-$ROUND.json"
LANE2_OUT="$EX/lane-${LANE2_FAMILY}-round-$ROUND.json"
rm -f "$LANE1_OUT" "$LANE2_OUT"

# --- dispatch the two cross-family lanes (fresh context each round) ----------
# `gc session submit <target> <text>` is the native semantic-delivery verb (the
# out-of-box gap map found it is the ONLY reliable way to advance a lane). We
# reuse mol-review-quorum's durable review-quorum.lane.v1 SCHEMA, but dispatch
# directly rather than slinging the core formula, because its fixed lane prompts
# cannot carry our per-round nonce / custom output path / cross-family RBAC
# (honest composition gap — see README §Gaps).
log "dispatching lane1=$LANE1_TARGET ($LANE1_FAMILY) + lane2=$LANE2_TARGET ($LANE2_FAMILY)"
submit_target "$LANE1_TARGET" "$(build_request "$LANE1_FAMILY-lane" "$LANE1_FAMILY" "$LANE1_OUT")" >/dev/null 2>&1 || log "lane1 submit reported an error (continuing; timeout branch handles degradation)"
submit_target "$LANE2_TARGET" "$(build_request "$LANE2_FAMILY-lane" "$LANE2_FAMILY" "$LANE2_OUT")" >/dev/null 2>&1 || log "lane2 submit reported an error (continuing; timeout branch handles degradation)"

# --- bounded wait for both lane JSONs (transient loss => DEGRADED, not fail) --
waited=0
while [ "$waited" -lt "$WAIT_SECS" ]; do
  n=0
  [ -s "$LANE1_OUT" ] && jq -e '.verdict' "$LANE1_OUT" >/dev/null 2>&1 && n=$((n+1))
  [ -s "$LANE2_OUT" ] && jq -e '.verdict' "$LANE2_OUT" >/dev/null 2>&1 && n=$((n+1))
  [ "$n" -ge 2 ] && break
  sleep 5; waited=$((waited+5))
done

# --- deterministic verdict (finalize.sh) -------------------------------------
PAWL="$EX/pawl-verdict-round-$ROUND.json"
ATTEMPT="$EX/review-attempt-round-$ROUND.json"
LANE_FILES=()
[ -s "$LANE1_OUT" ] && LANE_FILES+=("$LANE1_OUT")
[ -s "$LANE2_OUT" ] && LANE_FILES+=("$LANE2_OUT")

AUTHOR_SESSION="$(bdq list --json --limit 200 2>/dev/null | jq -r '[(if type=="array" then . else [.] end)[] | select((.metadata.template // "")=="builder")] | last | .metadata.session_name // empty' 2>/dev/null || true)"

"$FINALIZE" \
  --nonce "$NONCE" --round "$ROUND" --subject "$QUEST" --base-ref "main" \
  --expected-families "$LANE1_FAMILY,$LANE2_FAMILY" \
  --head-sha "$HEAD_SHA" --author "${AUTHOR_SESSION:-builder}" \
  --out "$PAWL" --attempt-out "$ATTEMPT" --evidence-dir "$EX/evidence-round-$ROUND" -- "${LANE_FILES[@]:-}"
FIN_EXIT=$?
if [ "$FIN_EXIT" -eq 3 ]; then
  DISPOSITION="DEGRADED"
  FCLASS="$(jq -r '.failure_class' "$ATTEMPT" 2>/dev/null || echo transient)"
  FREASON="$(jq -r '.failure_reason' "$ATTEMPT" 2>/dev/null || echo finalize_error)"
  log "review attempt round $ROUND: $DISPOSITION (class=$FCLASS) artifact=$ATTEMPT"
else
  cp "$PAWL" "$EX/pawl-verdict.json" 2>/dev/null || true
  DISPOSITION="$(jq -r '.disposition' "$PAWL" 2>/dev/null || echo REFUTED)"
  FCLASS="$( [ "$FIN_EXIT" -eq 0 ] && echo none || echo hard )"
  FREASON="$(printf '%s' "$DISPOSITION" | tr '[:upper:]' '[:lower:]')"
  log "verdict round $ROUND: $DISPOSITION (class=$FCLASS) artifact=$PAWL"
fi

case "$FIN_EXIT" in
  0)  # CONFIRMED — stamp the evidence-bound work record and CLOSE (exit 0)
    bdq update "$GC_BEAD_ID" \
      --set-metadata "gc.outcome=pass" \
      --set-metadata "gc.work_outcome=shipped" \
      --set-metadata "gc.work_branch=$BRANCH" \
      --set-metadata "gc.work_commit=$HEAD_SHA" \
      --set-metadata "gc.work_verification=membrane/$QUEST/runs/$RUN_ID/pawl-verdict-round-$ROUND.json" >/dev/null 2>&1 || true
    log "CONFIRMED — close authorized (a human merges $BRANCH)"
    exit 0 ;;
  3)  retry_exit transient "$FREASON" ;;   # DEGRADED — native GC consumes failed checks
  *)  retry_exit hard "$FREASON" ;;         # REFUTED — respawn builder
esac
