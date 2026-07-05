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
#      builder, consume an attempt); transient lane loss DEGRADES (retry the
#      lane, DO NOT consume an attempt) — degradation is never a false refute.
#
# It NEVER merges, pushes, or touches any branch. A human merges.
#
# Env from the dispatcher (internal/convergence/condition.go):
#   GC_BEAD_ID (iteration bead), GC_ITERATION, GC_MAX_ITERATIONS, GC_CITY_PATH.
# Overridable knobs (city may set; defaults assume [imports.agentops-membrane]):
#   MEMBRANE_LANE1_TARGET / MEMBRANE_LANE1_FAMILY   (default agentops-membrane.verifier / gpt)
#   MEMBRANE_LANE2_TARGET / MEMBRANE_LANE2_FAMILY   (default agentops-membrane.agy-verifier / gemini)
#   MEMBRANE_QUEST_ROOT (default $GC_CITY_PATH/quests)   MEMBRANE_WAIT_SECS (default 300)
set -u

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FINALIZE="$HERE/finalize.sh"

CITY="${GC_CITY_PATH:-$PWD}"
GC_BIN="${GC_BIN:-$(command -v gc || echo gc)}"
ROUND="${GC_ITERATION:-1}"
MAXR="${GC_MAX_ITERATIONS:-3}"
WAIT_SECS="${MEMBRANE_WAIT_SECS:-300}"
QUEST_ROOT="${MEMBRANE_QUEST_ROOT:-$CITY/quests}"

LANE1_TARGET="${MEMBRANE_LANE1_TARGET:-agentops-membrane.verifier}"
LANE1_FAMILY="${MEMBRANE_LANE1_FAMILY:-gpt}"
LANE2_TARGET="${MEMBRANE_LANE2_TARGET:-agentops-membrane.agy-verifier}"
LANE2_FAMILY="${MEMBRANE_LANE2_FAMILY:-gemini}"

log() { echo "[membrane-gate r$ROUND] $*" >&2; }
bdq() { (cd "$CITY" && bd "$@"); }
bead_json() { bdq show "$1" --json 2>/dev/null | jq 'if type=="array" then .[0] else . end'; }

# --- stamp the iteration bead's failure class then exit non-zero (retry) -----
# transient => native KindRetry re-spawns WITHOUT counting a refute (bead .1
# degradation-awareness); hard => counts as a refute round.
retry_exit() {  # $1=class(transient|hard) $2=reason
  bdq update "$GC_BEAD_ID" \
    --set-metadata "gc.outcome=fail" \
    --set-metadata "gc.failure_class=$1" \
    --set-metadata "gc.failure_reason=$2" >/dev/null 2>&1 || true
  log "NOT CONFIRMED: failure_class=$1 reason=$2 -> step fails, dispatcher decides retry"
  exit 1
}

# --- resolve quest metadata from the iteration bead (native: via bd) ---------
QUEST="$(bead_json "$GC_BEAD_ID" | jq -r '.metadata.quest // empty')"
if [ -z "$QUEST" ]; then
  log "FAIL-CLOSED: iteration bead $GC_BEAD_ID carries no quest metadata"
  retry_exit hard gate_no_quest_metadata
fi
REPO="$QUEST_ROOT/$QUEST"
BRANCH="quest/$QUEST"
EX="$CITY/membrane/$QUEST"
mkdir -p "$EX"

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
submit_lane() {  # $1=target $2=request-text
  "$GC_BIN" --city "$CITY" session submit "$1" "$2" 2>&1
}
log "dispatching lane1=$LANE1_TARGET ($LANE1_FAMILY) + lane2=$LANE2_TARGET ($LANE2_FAMILY)"
submit_lane "$LANE1_TARGET" "$(build_request "$LANE1_FAMILY-lane" "$LANE1_FAMILY" "$LANE1_OUT")" >/dev/null 2>&1 || log "lane1 submit reported an error (continuing; timeout branch handles degradation)"
submit_lane "$LANE2_TARGET" "$(build_request "$LANE2_FAMILY-lane" "$LANE2_FAMILY" "$LANE2_OUT")" >/dev/null 2>&1 || log "lane2 submit reported an error (continuing; timeout branch handles degradation)"

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
LANE_FILES=()
[ -s "$LANE1_OUT" ] && LANE_FILES+=("$LANE1_OUT")
[ -s "$LANE2_OUT" ] && LANE_FILES+=("$LANE2_OUT")

AUTHOR_SESSION="$(bdq list --json --limit 200 2>/dev/null | jq -r '[(if type=="array" then . else [.] end)[] | select((.metadata.template // "")=="builder")] | last | .metadata.session_name // empty' 2>/dev/null || true)"

"$FINALIZE" \
  --nonce "$NONCE" --round "$ROUND" --subject "$QUEST" --base-ref "main" \
  --expected-families "$LANE1_FAMILY,$LANE2_FAMILY" \
  --head-sha "$HEAD_SHA" --author "${AUTHOR_SESSION:-builder}" \
  --out "$PAWL" --evidence-dir "$EX" -- "${LANE_FILES[@]:-}"
FIN_EXIT=$?
cp "$PAWL" "$EX/pawl-verdict.json" 2>/dev/null || true
DISPOSITION="$(jq -r '.disposition' "$PAWL" 2>/dev/null || echo REFUTED)"
FCLASS="$(jq -r '.failure_class' "$PAWL" 2>/dev/null || echo hard)"
FREASON="$(jq -r '.failure_reason' "$PAWL" 2>/dev/null || echo finalize_error)"
log "verdict round $ROUND: $DISPOSITION (class=$FCLASS) artifact=$PAWL"

case "$FIN_EXIT" in
  0)  # CONFIRMED — stamp the evidence-bound work record and CLOSE (exit 0)
    bdq update "$GC_BEAD_ID" \
      --set-metadata "gc.outcome=pass" \
      --set-metadata "gc.work_outcome=shipped" \
      --set-metadata "gc.work_branch=$BRANCH" \
      --set-metadata "gc.work_commit=$HEAD_SHA" \
      --set-metadata "gc.work_verification=membrane/$QUEST/pawl-verdict-round-$ROUND.json" >/dev/null 2>&1 || true
    log "CONFIRMED — close authorized (a human merges $BRANCH)"
    exit 0 ;;
  3)  retry_exit transient "$FREASON" ;;   # DEGRADED — no attempt consumed
  *)  retry_exit hard "$FREASON" ;;         # REFUTED — respawn builder
esac
