#!/usr/bin/env bats

setup() {
  PACK="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  REPO_ROOT="$(cd "$PACK/../.." && pwd)"
  GATE="$PACK/membrane/close-gate.sh"
  CITY="$BATS_TEST_TMPDIR/city"
  QUEST="quest-1"
  QUEST_REPO="$CITY/quests/$QUEST"
  EX="$CITY/membrane/$QUEST/runs/workflow-1"
  FAKE_BIN="$BATS_TEST_TMPDIR/bin"
  BD_LOG="$BATS_TEST_TMPDIR/bd.log"
  GC_LOG="$BATS_TEST_TMPDIR/gc.log"
  HELPER_REQUEST_LOG="$BATS_TEST_TMPDIR/helper-request.log"
  mkdir -p "$QUEST_REPO" "$FAKE_BIN"

  git -C "$QUEST_REPO" init -q -b main
  git -C "$QUEST_REPO" config user.email test@example.com
  git -C "$QUEST_REPO" config user.name Test
  printf 'Given a contract\n' > "$QUEST_REPO/CONTRACT.md"
  git -C "$QUEST_REPO" add CONTRACT.md
  git -C "$QUEST_REPO" commit -qm contract
  git -C "$QUEST_REPO" checkout -qb "quest/$QUEST"
  printf 'implemented\n' > "$QUEST_REPO/work.txt"
  git -C "$QUEST_REPO" add work.txt
  git -C "$QUEST_REPO" commit -qm work
  git -C "$QUEST_REPO" checkout -q main

  # The close gate invokes bd only through its narrow show/list/update surface.
  cat > "$FAKE_BIN/bd" <<'SH'
#!/usr/bin/env bash
case "$1" in
  show)
    root=workflow-1
    [[ "$2" == iteration-2 ]] && root=workflow-2
    [[ "$2" == iteration-missing-root ]] && root=""
    jq -nc --arg root "$root" '[{metadata:{quest:"quest-1","gc.root_bead_id":$root,"gc.step_ref":"build.iteration.1"}}]' ;;
  list)
    if [[ " $* " == *" --include-gates "* ]]; then
      jq -nc --arg max "${FAKE_MAX_ITERATIONS:-6}" '[
        {metadata:{"gc.kind":"ralph","gc.root_bead_id":"workflow-1","gc.step_ref":"build","gc.max_attempts":$max}},
        {metadata:{"gc.kind":"ralph","gc.root_bead_id":"workflow-2","gc.step_ref":"build","gc.max_attempts":$max}}
      ]'
    else
      printf '%s\n' '[{"metadata":{"template":"builder","session_name":"builder-1"}}]'
    fi ;;
  update) printf '%s\n' "$*" >> "$BD_LOG" ;;
  *) exit 2 ;;
esac
SH
  chmod +x "$FAKE_BIN/bd"

  # Fake the live gc session lifecycle; the request itself supplies the nonce,
  # lane identity, and exact output path, as a real verifier receives it.
  cat > "$FAKE_BIN/gc" <<'SH'
#!/usr/bin/env bash
request="${!#}"
target="${@: -2:1}"
if [[ " $* " == *" session new "* ]]; then
  alias=""
  prev=""
  for arg in "$@"; do
    [[ "$prev" == --alias ]] && alias="$arg"
    prev="$arg"
  done
  id="helper-${alias}"
  printf 'new:%s:%s\n' "$alias" "$id" >> "$GC_LOG"
  jq -nc --arg id "$id" --arg alias "$alias" '{schema_version:"1",ok:true,session_id:$id,session_name:("s-"+$id),alias:$alias,template:"agentops-membrane.breaker-helper",transport:"tmux",work_dir:".",deferred_start:true,attached:false}'
  exit 0
fi
if [[ " $* " == *" session list "* ]]; then
  printf '{"sessions":[]}\n'
  exit 0
fi
if [[ " $* " == *" session close "* ]]; then
  printf 'close:%s\n' "${!#}" >> "$GC_LOG"
  [[ -n "${FAKE_CLOSE_FAIL:-}" ]] && exit 1
  exit 0
fi
printf 'submit:%s\n' "$target" >> "$GC_LOG"
if [[ "$request" == BREAKER\ HELPER* ]]; then
  printf '%s\n' "$request" > "$HELPER_REQUEST_LOG"
  out="$(printf '%s\n' "$request" | sed -n 's/^  \(.*breaker-helper-round-.*\.json\)$/\1/p' | head -1)"
  nonce="$(printf '%s\n' "$request" | sed -n 's/.*agentops_nonce="\([^"]*\)".*/\1/p' | head -1)"
  [[ "${FAKE_HELPER_MODE:-valid}" == no-output ]] && exit 0
  if [[ "${FAKE_HELPER_MODE:-valid}" == invalid ]]; then
    printf '{}\n' > "$out"
    exit 0
  fi
  outcome="${FAKE_HELPER_OUTCOME:-UNSTUCK}"
  approach="apply the evidence-bound alternate implementation"
  [[ "$outcome" == ESCALATE ]] && approach=""
  mkdir -p "$(dirname "$out")"
  jq -n --arg outcome "$outcome" --arg approach "$approach" --arg nonce "$nonce" '{
    schema_version:"agentops-breaker-helper.v1",outcome:$outcome,
    new_approach:$approach,reason:"bounded helper decision",
    evidence:["pawl-verdict-round-5.json:disposition"],agentops_nonce:$nonce
  }' > "$out"
  exit 0
fi
if [[ "${FAKE_GC_MODE:-confirmed}" == degraded && "$target" == *agy* ]]; then exit 0; fi
out="$(printf '%s\n' "$request" | sed -n 's/^  \(.*lane-.*-round-.*\.json\)$/\1/p' | head -1)"
nonce="$(printf '%s\n' "$request" | sed -n 's/.*agentops_nonce="\([^"]*\)".*/\1/p' | head -1)"
lane="$(printf '%s\n' "$request" | sed -n 's/.*Lane: \([^ ]*\) (\([^)]*\)).*/\1/p' | head -1)"
provider="$(printf '%s\n' "$request" | sed -n 's/.*Lane: \([^ ]*\) (\([^)]*\)).*/\2/p' | head -1)"
mkdir -p "$(dirname "$out")"
verdict=pass
findings='[]'
if [[ "${FAKE_GC_MODE:-confirmed}" == refuted ]]; then
  verdict=fail
  findings='[{"title":"contract defect","body":"missing behavior","file":"work.txt","severity":"high"}]'
fi
jq -n --arg lane "$lane" --arg provider "$provider" --arg nonce "$nonce" \
  --arg verdict "$verdict" --argjson findings "$findings" '{
  lane_id:$lane,provider:$provider,model:($provider+"-model"),verdict:$verdict,summary:"reviewed files: 1",
  findings_count:($findings|length),findings:$findings,evidence:["work.txt:1"],usage:null,
  read_only_enforcement:{observed:true,enabled:true,passed:true,baseline_command:"git status",after_command:"git status"},
  mutations_delta:{changed:[]},failure_class:"none",failure_reason:"",agentops_nonce:$nonce
}' > "$out"
SH
  chmod +x "$FAKE_BIN/gc"
}

run_gate() {
  run env PATH="$FAKE_BIN:$PATH" BD_LOG="$BD_LOG" GC_LOG="$GC_LOG" \
    HELPER_REQUEST_LOG="$HELPER_REQUEST_LOG" \
    FAKE_MAX_ITERATIONS="${3:-3}" \
    FAKE_GC_MODE="${1:-confirmed}" FAKE_HELPER_OUTCOME="${4:-UNSTUCK}" \
    FAKE_HELPER_MODE="${FAKE_HELPER_MODE:-valid}" FAKE_CLOSE_FAIL="${FAKE_CLOSE_FAIL:-}" \
    GC_BEAD_ID="${5:-iteration-1}" GC_ITERATION="${2:-1}" GC_MAX_ITERATIONS="${GC_MAX_ITERATIONS_OVERRIDE:-${3:-3}}" GC_CITY_PATH="$CITY" \
    GC_BIN="$FAKE_BIN/gc" MEMBRANE_WAIT_SECS=0 MEMBRANE_HELPER_WAIT_SECS=0 MEMBRANE_QUEST_ROOT="$CITY/quests" \
    bash "$GATE"
}

@test "close gate publishes only a canonical terminal verdict" {
  run_gate confirmed
  [ "$status" -eq 0 ]
  [ -s "$EX/pawl-verdict-round-1.json" ]
  [ -s "$EX/pawl-verdict.json" ]
  cmp "$EX/pawl-verdict-round-1.json" "$EX/pawl-verdict.json"
  python3 - "$REPO_ROOT/schemas/pawl-verdict.v1.schema.json" "$EX/pawl-verdict.json" <<'PY'
import json, pathlib, sys
import jsonschema
schema=json.load(open(sys.argv[1])); verdict=json.load(open(sys.argv[2])); jsonschema.validate(verdict,schema)
for lane in verdict["refuters"]:
    p=pathlib.Path(lane["evidence"]); assert p.is_file() and p.stat().st_size > 0
PY
}

@test "close gate preserves the latest terminal verdict when review transport degrades" {
  mkdir -p "$EX"
  printf 'prior-terminal-verdict\n' > "$EX/pawl-verdict.json"
  before="$(shasum -a 256 "$EX/pawl-verdict.json" | awk '{print $1}')"
  run_gate degraded
  [ "$status" -ne 0 ]
  [ ! -e "$EX/pawl-verdict-round-1.json" ]
  [ -s "$EX/review-attempt-round-1.json" ]
  [ "$(jq -r '.outcome' "$EX/review-attempt-round-1.json")" = DEGRADED ]
  after="$(shasum -a 256 "$EX/pawl-verdict.json" | awk '{print $1}')"
  [ "$before" = "$after" ]
  grep -q 'gc.failure_class=transient' "$BD_LOG"
}

@test "pack documentation admits that native GC consumes every failed check attempt" {
  run rg -n --glob '!close-gate.bats' 'DO NOT consume an attempt|no([^A-Za-z]|\*|_)*attempt consumed|does not consume an attempt' \
    "$PACK/membrane" "$PACK/tests" "$PACK/README.md" "$PACK/QUICKSTART.md"
  [ "$status" -eq 1 ]
}

@test "attempt exhaustion dispatches exactly one executable helper" {
  run_gate refuted 5 6 UNSTUCK
  [ "$status" -ne 0 ]
  [ -s "$EX/breaker-helper-round-5.json" ]
  [ "$(jq -r '.outcome' "$EX/breaker-helper-round-5.json")" = UNSTUCK ]
  [ "$(grep -c '^new:breaker-quest-1-r5-.*:helper-breaker-quest-1-r5-' "$GC_LOG")" -eq 1 ]
  helper_id="$(jq -r '.session_id' "$EX/breaker-helper-session-round-5.json")"
  [ "$(grep -c "^submit:$helper_id$" "$GC_LOG")" -eq 1 ]
  [ "$(grep -c "^close:$helper_id$" "$GC_LOG")" -eq 1 ]
  ! grep -q '^submit:agentops-membrane.breaker-helper$' "$GC_LOG"
  ! grep -q '^reset:' "$GC_LOG"
  grep -q 'contract defect' "$HELPER_REQUEST_LOG"
  grep -q 'missing behavior' "$HELPER_REQUEST_LOG"
  grep -q 'gc.failure_reason=refuted' "$BD_LOG"
}

@test "graph.v2 zero max-iterations env recovers the control-bead budget" {
  export GC_MAX_ITERATIONS_OVERRIDE=0
  run_gate refuted 5 6 UNSTUCK
  [ "$status" -ne 0 ]
  [ "$(jq -r '.outcome' "$EX/breaker-helper-round-5.json")" = UNSTUCK ]
  [ "$(grep -c '^new:' "$GC_LOG")" -eq 1 ]
}

@test "positive env and control-bead budget disagreement fails closed" {
  export GC_MAX_ITERATIONS_OVERRIDE=5
  run_gate confirmed 1 6 UNSTUCK
  [ "$status" -ne 0 ]
  grep -q 'gc.failure_reason=gate_attempt_budget_mismatch' "$BD_LOG"
  [ ! -s "$GC_LOG" ]
}

@test "invalid helper output escalates and still closes the fresh session" {
  export FAKE_HELPER_MODE=invalid
  run_gate refuted 5 6 UNSTUCK
  [ "$status" -ne 0 ]
  [ "$(jq -r '.outcome' "$EX/breaker-helper-round-5.json")" = ESCALATE ]
  helper_id="$(jq -r '.session_id' "$EX/breaker-helper-session-round-5.json")"
  grep -q "^close:$helper_id$" "$GC_LOG"
}

@test "missing helper output escalates and still closes the fresh session" {
  export FAKE_HELPER_MODE=no-output
  run_gate refuted 5 6 UNSTUCK
  [ "$status" -ne 0 ]
  [ "$(jq -r '.outcome' "$EX/breaker-helper-round-5.json")" = ESCALATE ]
  [ "$(jq -r '.reason' "$EX/breaker-helper-round-5.json")" = helper_unavailable_or_invalid ]
  helper_id="$(jq -r '.session_id' "$EX/breaker-helper-session-round-5.json")"
  [ "$(grep -c "^close:$helper_id$" "$GC_LOG")" -eq 1 ]
}

@test "helper close failure escalates without retrying the close" {
  export FAKE_CLOSE_FAIL=1
  run_gate refuted 5 6 UNSTUCK
  [ "$status" -ne 0 ]
  [ "$(jq -r '.outcome' "$EX/breaker-helper-round-5.json")" = ESCALATE ]
  [ "$(jq -r '.reason' "$EX/breaker-helper-round-5.json")" = helper_session_close_failed ]
  helper_id="$(jq -r '.session_id' "$EX/breaker-helper-session-round-5.json")"
  [ "$(grep -c "^close:$helper_id$" "$GC_LOG")" -eq 1 ]
}

@test "re-entry closes a helper that wrote its outcome before a crash" {
  run_gate refuted 5 6 UNSTUCK
  [ "$status" -ne 0 ]
  receipt="$EX/breaker-helper-session-round-5.json"
  helper_id="$(jq -r '.session_id' "$receipt")"
  rm -f "$receipt.closed"
  : > "$GC_LOG"
  run_gate refuted 5 6 UNSTUCK
  [ "$status" -ne 0 ]
  [ "$(grep -c "^close:$helper_id$" "$GC_LOG")" -eq 1 ]
  ! grep -q '^new:' "$GC_LOG"
  ! grep -q '^submit:helper-' "$GC_LOG"
  [ -s "$receipt.closed" ]
}

@test "valid persisted helper outcome is idempotent and performs no second session operation" {
  run_gate refuted 5 6 UNSTUCK
  [ "$status" -ne 0 ]
  : > "$GC_LOG"
  run_gate refuted 5 6 UNSTUCK
  [ "$status" -ne 0 ]
  ! grep -q '^new:' "$GC_LOG"
  ! grep -q '^submit:helper-' "$GC_LOG"
  ! grep -q '^close:helper-' "$GC_LOG"
}

@test "re-slinging the same quest cannot replay a prior run helper outcome" {
  run_gate refuted 5 6 UNSTUCK iteration-1
  [ "$status" -ne 0 ]
  [ "$(jq -r '.outcome' "$EX/breaker-helper-round-5.json")" = UNSTUCK ]
  : > "$GC_LOG"
  ex2="$CITY/membrane/$QUEST/runs/workflow-2"
  run_gate refuted 5 6 ESCALATE iteration-2
  [ "$status" -ne 0 ]
  [ "$(jq -r '.outcome' "$ex2/breaker-helper-round-5.json")" = ESCALATE ]
  [ "$(grep -c '^new:' "$GC_LOG")" -eq 1 ]
  helper_id="$(jq -r '.session_id' "$ex2/breaker-helper-session-round-5.json")"
  [ "$(grep -c "^submit:$helper_id$" "$GC_LOG")" -eq 1 ]
  [ "$(grep -c "^close:$helper_id$" "$GC_LOG")" -eq 1 ]
}

@test "missing workflow root fails closed before writing unscoped evidence" {
  run_gate confirmed 1 6 UNSTUCK iteration-missing-root
  [ "$status" -ne 0 ]
  grep -q 'gc.failure_reason=gate_no_run_metadata' "$BD_LOG"
  [ ! -d "$CITY/membrane/$QUEST/runs/" ]
}

@test "HELPER-UNSTUCK gets one recovery attempt that must re-earn CONFIRMED" {
  run_gate refuted 5 6 UNSTUCK
  [ "$status" -ne 0 ]
  : > "$GC_LOG"
  run_gate confirmed 6 6 UNSTUCK
  [ "$status" -eq 0 ]
  [ "$(jq -r '.disposition' "$EX/pawl-verdict-round-6.json")" = CONFIRMED ]
  [ "$(grep -c 'agentops-membrane.breaker-helper' "$GC_LOG" || true)" -eq 0 ]
  grep -q 'agentops-membrane.verifier' "$GC_LOG"
}

@test "HELPER-ESCALATE terminates recovery without another review" {
  run_gate refuted 5 6 ESCALATE
  [ "$status" -ne 0 ]
  [ "$(jq -r '.outcome' "$EX/breaker-helper-round-5.json")" = ESCALATE ]
  : > "$GC_LOG"
  run_gate confirmed 6 6 ESCALATE
  [ "$status" -ne 0 ]
  grep -q 'gc.failure_reason=helper_escalate' "$BD_LOG"
  [ ! -s "$GC_LOG" ]
}

@test "reviewer plus helper wait budget fits inside the formula check timeout" {
  reviewer="$(sed -n 's/^WAIT_SECS="${MEMBRANE_WAIT_SECS:-\([0-9][0-9]*\)}"$/\1/p' "$GATE")"
  helper="$(sed -n 's/^HELPER_WAIT_SECS="${MEMBRANE_HELPER_WAIT_SECS:-\([0-9][0-9]*\)}"$/\1/p' "$GATE")"
  lease="$(sed -n 's/^GATE_TIMEOUT_SECS="${MEMBRANE_GATE_TIMEOUT_SECS:-\([0-9][0-9]*\)}"$/\1/p' "$GATE")"
  safety="$(sed -n 's/^GATE_SAFETY_SECS="${MEMBRANE_GATE_SAFETY_SECS:-\([0-9][0-9]*\)}"$/\1/p' "$GATE")"
  [ -n "$reviewer" ] && [ -n "$helper" ] && [ -n "$lease" ] && [ -n "$safety" ]
  [ $((reviewer + helper + safety)) -le "$lease" ]
  grep -q 'HELPER_WAIT_BUDGET=' "$GATE"
  grep -q 'HELPER_WAIT_SECS="$HELPER_WAIT_BUDGET"' "$GATE"
  grep -q 'timeout = "8m"' "$PACK/formulas/membrane-quest.toml"
}
