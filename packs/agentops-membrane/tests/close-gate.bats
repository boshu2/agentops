#!/usr/bin/env bats

setup() {
  PACK="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  REPO_ROOT="$(cd "$PACK/../.." && pwd)"
  GATE="$PACK/membrane/close-gate.sh"
  CITY="$BATS_TEST_TMPDIR/city"
  QUEST="quest-1"
  QUEST_REPO="$CITY/quests/$QUEST"
  EX="$CITY/membrane/$QUEST"
  FAKE_BIN="$BATS_TEST_TMPDIR/bin"
  BD_LOG="$BATS_TEST_TMPDIR/bd.log"
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
  show) printf '%s\n' '[{"metadata":{"quest":"quest-1"}}]' ;;
  list) printf '%s\n' '[{"metadata":{"template":"builder","session_name":"builder-1"}}]' ;;
  update) printf '%s\n' "$*" >> "$BD_LOG" ;;
  *) exit 2 ;;
esac
SH
  chmod +x "$FAKE_BIN/bd"

  # Fake only the live gc session-submit seam; the request itself supplies the
  # nonce, lane identity, and exact output path, as a real verifier receives it.
  cat > "$FAKE_BIN/gc" <<'SH'
#!/usr/bin/env bash
request="${!#}"
target="${@: -2:1}"
if [[ "${FAKE_GC_MODE:-confirmed}" == degraded && "$target" == *agy* ]]; then exit 0; fi
out="$(printf '%s\n' "$request" | sed -n 's/^  \(.*lane-.*-round-.*\.json\)$/\1/p' | head -1)"
nonce="$(printf '%s\n' "$request" | sed -n 's/.*agentops_nonce="\([^"]*\)".*/\1/p' | head -1)"
lane="$(printf '%s\n' "$request" | sed -n 's/.*Lane: \([^ ]*\) (\([^)]*\)).*/\1/p' | head -1)"
provider="$(printf '%s\n' "$request" | sed -n 's/.*Lane: \([^ ]*\) (\([^)]*\)).*/\2/p' | head -1)"
mkdir -p "$(dirname "$out")"
jq -n --arg lane "$lane" --arg provider "$provider" --arg nonce "$nonce" '{
  lane_id:$lane,provider:$provider,model:($provider+"-model"),verdict:"pass",summary:"reviewed files: 1",
  findings_count:0,findings:[],evidence:["work.txt:1"],usage:null,
  read_only_enforcement:{observed:true,enabled:true,passed:true,baseline_command:"git status",after_command:"git status"},
  mutations_delta:{changed:[]},failure_class:"none",failure_reason:"",agentops_nonce:$nonce
}' > "$out"
SH
  chmod +x "$FAKE_BIN/gc"
}

run_gate() {
  run env PATH="$FAKE_BIN:$PATH" BD_LOG="$BD_LOG" FAKE_GC_MODE="${1:-confirmed}" \
    GC_BEAD_ID=iteration-1 GC_ITERATION=1 GC_MAX_ITERATIONS=3 GC_CITY_PATH="$CITY" \
    GC_BIN="$FAKE_BIN/gc" MEMBRANE_WAIT_SECS=0 MEMBRANE_QUEST_ROOT="$CITY/quests" \
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
