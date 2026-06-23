#!/usr/bin/env bats
# pawl.sh metrics (ml8.6) — SLO surface over the recorded routes: p50/p95 round-trip
# latency + agreement rate. Reads the append-only metrics.jsonl cmd_route writes. Tests
# run pawl.sh from a throwaway ROOT (a non-git temp dir, so ROOT=pwd) with a synthetic
# metrics file — no live model routes needed.

setup() {
  REPO="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  PAWL="$REPO/scripts/pawl.sh"
  TMP="$(mktemp -d)"
  mkdir -p "$TMP/.agents/pawl"
  MF="$TMP/.agents/pawl/metrics.jsonl"
}
teardown() { rm -rf "$TMP"; }

_seed3() {  # 3 routes: latencies 30/74/120; 2 agree, 1 disagree
  cat > "$MF" <<'EOF'
{"ts":"2026-06-23T21:00:00Z","bead":"a","latency_s":30,"opus":"CONFIRMED","codex":"CONFIRMED","disposition":"CONFIRMED","agreement":"agree"}
{"ts":"2026-06-23T21:05:00Z","bead":"b","latency_s":74,"opus":"CONFIRMED","codex":"REFUTED","disposition":"REFUTED","agreement":"disagree"}
{"ts":"2026-06-23T21:10:00Z","bead":"c","latency_s":120,"opus":"CONFIRMED","codex":"CONFIRMED","disposition":"CONFIRMED","agreement":"agree"}
EOF
}

@test "pawl metrics: p50/p95 latency + agreement rate over >=3 routes (text)" {
  _seed3
  run bash -c "cd '$TMP' && bash '$PAWL' metrics"
  [ "$status" -eq 0 ]
  [[ "$output" == *"3 routed beads"* ]]
  [[ "$output" == *"p50=74s"* ]]
  [[ "$output" == *"p95=120s"* ]]
  [[ "$output" == *"agreement 2/3"* ]]
  [[ "$output" == *"disagreements=1"* ]]
}

@test "pawl metrics: --json emits the SLO object (p50/p95/agreement_rate/disagreements)" {
  _seed3
  run bash -c "cd '$TMP' && bash '$PAWL' metrics --json"
  [ "$status" -eq 0 ]
  [ "$(echo "$output" | jq -r '.routes')" -eq 3 ]
  [ "$(echo "$output" | jq -r '.latency_p50_s')" -eq 74 ]
  [ "$(echo "$output" | jq -r '.latency_p95_s')" -eq 120 ]
  [ "$(echo "$output" | jq -r '.disagreements')" -eq 1 ]
  [ "$(echo "$output" | jq -r '.agreement_rate')" = "0.667" ]
}

@test "pawl metrics: a disagreement is counted + inspectable (ml8.6 acceptance)" {
  _seed3
  run bash -c "cd '$TMP' && bash '$PAWL' metrics --json"
  [ "$(echo "$output" | jq -r '.disagreements')" -eq 1 ]
  [ "$(echo "$output" | jq -r '.agree')" -eq 2 ]
}

@test "pawl metrics: no routes recorded yet is a clean no-op (exit 0)" {
  run bash -c "cd '$TMP' && bash '$PAWL' metrics"
  [ "$status" -eq 0 ]
  [[ "$output" == *"no routed beads recorded yet"* ]]
}

@test "pawl metrics: --json with no routes emits routes:0" {
  run bash -c "cd '$TMP' && bash '$PAWL' metrics --json"
  [ "$status" -eq 0 ]
  [ "$(echo "$output" | jq -r '.routes')" -eq 0 ]
}

@test "pawl metrics: a corrupt/partial line does NOT crash — fail-soft over the valid rows (codex catch)" {
  _seed3
  printf '{"ts":"partial","latency_s":  \n' >> "$MF"   # a corrupt/half-written append
  printf 'not json at all\n' >> "$MF"
  run bash -c "cd '$TMP' && bash '$PAWL' metrics --json"
  [ "$status" -eq 0 ]
  [ "$(echo "$output" | jq -r '.routes')" -eq 3 ]   # the 3 valid rows still computed; corrupt skipped
}
