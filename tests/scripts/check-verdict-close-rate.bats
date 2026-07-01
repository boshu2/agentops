#!/usr/bin/env bats
# age-wedge-all-in-dyr0.4: warn-first verdict-close-rate gate over the br
# ledger JSONL via scripts/check-verdict-close-rate.sh

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/check-verdict-close-rate.sh"
  TMP="$(mktemp -d)"
  export BEADS_DIR="$TMP/beads"
  mkdir -p "$BEADS_DIR"
  # Isolate from operator env so a real ratchet setting can't skew fixtures.
  unset AGENTOPS_VERDICT_CLOSE_RATE_SKIP AGENTOPS_VERDICT_CLOSE_RATE_STRICT
  unset AGENTOPS_VERDICT_CLOSE_RATE_THRESHOLD AGENTOPS_VERDICT_CLOSE_RATE_WINDOW
}

teardown() {
  rm -rf "$TMP"
}

# issue <id> <status> <closed_at> <close_reason> — one fixture JSONL line.
issue() {
  printf '{"id":"%s","title":"t","status":"%s","closed_at":"%s","close_reason":"%s"}\n' \
    "$1" "$2" "$3" "$4" >> "$BEADS_DIR/issues.jsonl"
}

seed_mixed_ledger() {
  # 4 closes in the window: 2 stamped (CONFIRMED + UNVERIFIED both count as
  # stamped — the stamp is what makes the close greppable), 2 prose-only.
  issue ag-1 closed 2026-07-01T04:00:00Z "Landed abc [verdict:1a1a1a1:CONFIRMED]"
  issue ag-2 closed 2026-07-01T03:00:00Z "Done, looks good"
  issue ag-3 closed 2026-07-01T02:00:00Z "Shipped [verdict:3c3c3c3:UNVERIFIED]"
  issue ag-4 closed 2026-07-01T01:00:00Z "prose close"
  issue ag-5 open "" ""
}

@test "warn-only default: below-threshold rate warns but exits 0" {
  seed_mixed_ledger
  run bash "$SCRIPT" --threshold 90
  [ "$status" -eq 0 ]
  [[ "$output" == *"WARN"* ]]
  [[ "$output" == *"2/4"* ]]
  [[ "$output" == *"50%"* ]]
  [[ "$output" == *"ao done"* ]]
}

@test "default threshold 0 is an informational PASS with the measured rate" {
  seed_mixed_ledger
  run bash "$SCRIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS"* ]]
  [[ "$output" == *"2/4"* ]]
}

@test "strict below threshold fails with exit 1" {
  seed_mixed_ledger
  run bash "$SCRIPT" --strict --threshold 60
  [ "$status" -eq 1 ]
  [[ "$output" == *"FAIL"* ]]
  [[ "$output" == *"50%"* ]]
}

@test "strict at/above threshold passes" {
  seed_mixed_ledger
  run bash "$SCRIPT" --strict --threshold 50
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS"* ]]
}

@test "window limits the sample to the newest N closes" {
  seed_mixed_ledger
  # newest 2 closes = ag-1 (stamped) + ag-2 (prose) => 1/2 = 50%
  run bash "$SCRIPT" --window 2 --strict --threshold 51
  [ "$status" -eq 1 ]
  [[ "$output" == *"1/2"* ]]
}

@test "JSONL is last-wins per id: a re-close supersedes the earlier line" {
  issue ag-9 closed 2026-07-01T01:00:00Z "prose close"
  issue ag-9 closed 2026-07-01T02:00:00Z "re-closed [verdict:9f9f9f9:CONFIRMED]"
  run bash "$SCRIPT" --strict --threshold 100
  [ "$status" -eq 0 ]
  [[ "$output" == *"1/1"* ]]
}

@test "open issues never count in the window" {
  issue ag-open open "" ""
  issue ag-c closed 2026-07-01T01:00:00Z "x [verdict:abcabca:waived-trivial]"
  run bash "$SCRIPT" --json
  [ "$status" -eq 0 ]
  [[ "$output" == *'"total":1'* ]]
  [[ "$output" == *'"stamped":1'* ]]
  [[ "$output" == *'"result":"PASS"'* ]]
}

@test "absent ledger file skips cleanly" {
  rm -f "$BEADS_DIR/issues.jsonl"
  run bash "$SCRIPT" --strict --threshold 100
  [ "$status" -eq 0 ]
  [[ "$output" == *"SKIP"* ]]
  [[ "$output" == *"ledger absent"* ]]
}

@test "unresolvable ledger dir skips cleanly (no BEADS_DIR, ao unavailable)" {
  unset BEADS_DIR
  # Stub `ao` that fails, ahead of any real one on PATH.
  mkdir -p "$TMP/bin"
  printf '#!/bin/sh\nexit 1\n' > "$TMP/bin/ao"
  chmod +x "$TMP/bin/ao"
  PATH="$TMP/bin:$PATH" run bash "$SCRIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"SKIP"* ]]
  [[ "$output" == *"unresolvable"* ]]
}

@test "jq absent skips cleanly" {
  seed_mixed_ledger
  # PATH with the essentials but no jq.
  mkdir -p "$TMP/bin"
  for tool in bash sh grep sed printf echo; do
    p="$(command -v "$tool" 2>/dev/null || true)"
    [ -n "$p" ] && ln -s "$p" "$TMP/bin/$tool" 2>/dev/null || true
  done
  PATH="$TMP/bin" run bash "$SCRIPT" --strict --threshold 100
  [ "$status" -eq 0 ]
  [[ "$output" == *"SKIP"* ]]
  [[ "$output" == *"jq"* ]]
}

@test "operator SKIP env short-circuits" {
  seed_mixed_ledger
  AGENTOPS_VERDICT_CLOSE_RATE_SKIP=1 run bash "$SCRIPT" --strict --threshold 100
  [ "$status" -eq 0 ]
  [[ "$output" == *"SKIP"* ]]
}

@test "empty window (no closed beads) skips cleanly" {
  issue ag-open open "" ""
  run bash "$SCRIPT" --strict --threshold 100
  [ "$status" -eq 0 ]
  [[ "$output" == *"SKIP"* ]]
  [[ "$output" == *"no closed beads"* ]]
}

@test "--json emits the machine shape with rate and threshold" {
  seed_mixed_ledger
  run bash "$SCRIPT" --json --strict --threshold 60
  [ "$status" -eq 1 ]
  [[ "$output" == *'"stamped":2'* ]]
  [[ "$output" == *'"total":4'* ]]
  [[ "$output" == *'"rate_pct":50'* ]]
  [[ "$output" == *'"threshold_pct":60'* ]]
  [[ "$output" == *'"result":"FAIL"'* ]]
}

@test "unknown argument exits 2" {
  run bash "$SCRIPT" --nope
  [ "$status" -eq 2 ]
}
