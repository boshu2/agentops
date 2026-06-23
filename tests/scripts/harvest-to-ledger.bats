#!/usr/bin/env bats
# Regression tests for evals/membrane/harvest-2026-06-22/harvest-to-ledger.sh — turns
# a membrane-eval scorecard into escape chains in an ISOLATED yield ledger. The
# fail-closed guards here once had a real fabrication-class bug (an IFS-collapse made
# it emit a DEGRADED task as an escape), so each is pinned: caught -> 2-verdict chain,
# escaped(miss) -> 3-verdict chain, degraded -> SKIPPED, true-done -> skipped. Uses
# the real `ao yield emit` against a throwaway ledger root (no models).

setup() {
  REPO="$BATS_TEST_DIRNAME/../.."
  SCRIPT="$REPO/evals/membrane/harvest-2026-06-22/harvest-to-ledger.sh"
  # Prefer an already-built ao; fall back to building one (the script does this too).
  AO="$REPO/cli/bin/ao"; [ -x "$AO" ] || AO="/tmp/ao-pawl"
  [ -x "$AO" ] || ( cd "$REPO/cli" && go build -o /tmp/ao-pawl ./cmd/ao && AO=/tmp/ao-pawl )
  export AGENTOPS_AO_BIN="$AO"
  FIX="$(mktemp -d)"
}
teardown() { rm -rf "$FIX"; }

# Build a one-task scorecard. $1=class-shape: caught|escaped|degraded|truedone
scorecard() {
  case "$1" in
    caught)   echo '{"per_task":[{"task":"t","oracle_pass":false,"verdict":"REFUTE","why":"wrong","degraded":false}]}' ;;
    escaped)  echo '{"per_task":[{"task":"t","oracle_pass":false,"verdict":"ACK","why":"looked ok","degraded":false}]}' ;;
    degraded) echo '{"per_task":[{"task":"t","oracle_pass":false,"verdict":"DRY","why":"","degraded":true}]}' ;;
    truedone) echo '{"per_task":[{"task":"t","oracle_pass":true,"verdict":"ACK","why":"ok","degraded":false}]}' ;;
  esac
}

ledger() { echo "$FIX/lroot/.agents/yield/yield-ledger.jsonl"; }
dispositions() { # disposition list from the ledger, in order
  python3 -c 'import json,sys
for line in open(sys.argv[1]):
    line=line.strip()
    if line:
        d=json.loads(line); b=d.get("body",d); print(b.get("disposition"))' "$(ledger)"
}

@test "a caught false-done emits a 2-verdict escape chain (producer CONFIRMED -> membrane REFUTED)" {
  scorecard caught > "$FIX/sc.json"
  run bash "$SCRIPT" "$FIX/sc.json" "$FIX/lroot" run-x
  [ "$status" -eq 0 ]
  [ "$(wc -l < "$(ledger)" | tr -d ' ')" = "2" ]
  run dispositions
  [ "${lines[0]}" = "CONFIRMED" ]
  [ "${lines[1]}" = "REFUTED" ]
}

@test "an escaped (membrane MISS) emits a 3-verdict chain ending in an oracle REFUTED" {
  scorecard escaped > "$FIX/sc.json"
  run bash "$SCRIPT" "$FIX/sc.json" "$FIX/lroot" run-x
  [ "$status" -eq 0 ]
  [ "$(wc -l < "$(ledger)" | tr -d ' ')" = "3" ]
  run dispositions
  [ "${lines[0]}" = "CONFIRMED" ]   # producer
  [ "${lines[1]}" = "CONFIRMED" ]   # membrane ACK = the wrong confirm
  [ "${lines[2]}" = "REFUTED" ]     # oracle ground-truth overturn
}

@test "a DEGRADED task is SKIPPED, never emitted as an escape (the IFS-collapse fabrication guard)" {
  scorecard degraded > "$FIX/sc.json"
  run bash "$SCRIPT" "$FIX/sc.json" "$FIX/lroot" run-x
  [ "$status" -eq 0 ]
  [[ "$output" == *"skipped (true-done/degraded): 1"* ]]
  # No ledger file/lines for a skipped task (nothing emitted).
  [ ! -s "$(ledger)" ]
}

@test "a true-done is skipped (only false-dones are escapes)" {
  scorecard truedone > "$FIX/sc.json"
  run bash "$SCRIPT" "$FIX/sc.json" "$FIX/lroot" run-x
  [ "$status" -eq 0 ]
  [[ "$output" == *"skipped (true-done/degraded): 1"* ]]
  [ ! -s "$(ledger)" ]
}
