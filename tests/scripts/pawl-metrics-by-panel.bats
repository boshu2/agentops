#!/usr/bin/env bats
# pawl.sh metrics per-panel breakdown (F7, age-pawl-intent-zhndq.7): `ao pawl metrics` must group
# routes by family-set and flag AGREEMENT-COLLAPSE (full-agreement <30% over >=5 routes) so a bad
# panel is a visible signal — the audit found tri (cc cod agy) reached full agreement 0/22 times
# while dual (cc cod) was fine, and nothing surfaced it. Tolerates legacy null-tier/family rows.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/pawl.sh"
  TMP="$(mktemp -d)"
  MF="$TMP/.agents/pawl/metrics.jsonl"; mkdir -p "$(dirname "$MF")"
  # A collapsing tri panel (0/6 agree) + a healthy dual panel (4/5 agree) + a legacy null row.
  {
    for i in 1 2 3 4 5 6; do printf '{"tier":"multi","families":"cc cod agy","latency_s":300,"agreement":"disagree"}\n'; done
    for i in 1 2 3 4;   do printf '{"tier":"multi","families":"cc cod","latency_s":90,"agreement":"agree"}\n'; done
    printf '{"tier":"multi","families":"cc cod","latency_s":90,"agreement":"disagree"}\n'
    printf '{"tier":null,"families":null,"latency_s":50,"agreement":"agree"}\n'
  } > "$MF"
}
teardown() { rm -rf "$TMP"; }

run_metrics() {
  bash -c 'PAWL_SESSION=x; source "$2" 2>/dev/null; ROOT="$1"; STATE_DIR=".agents/pawl"; cmd_metrics "$3" 2>&1' _ "$TMP" "$SCRIPT" "${1:-}"
}

@test "metrics --json: per-panel breakdown flags the collapsing tri panel, not the healthy dual" {
  run run_metrics --json
  [ "$status" -eq 0 ]
  # tri panel: 0/6 agree, collapse=true
  echo "$output" | python3 -c '
import json,sys
d=json.load(sys.stdin); bf=d["by_families"]
assert bf["cc cod agy"]["routes"]==6, bf
assert bf["cc cod agy"]["agree"]==0, bf
assert bf["cc cod agy"]["collapse"] is True, bf
# dual panel: 4/5 agree, NOT collapsed
assert bf["cc cod"]["agree"]==4 and bf["cc cod"]["routes"]==5, bf
assert bf["cc cod"]["collapse"] is False, bf
print("OK")
'
}

@test "metrics text: prints the by-panel section with the AGREEMENT-COLLAPSE flag" {
  run run_metrics
  [ "$status" -eq 0 ]
  [[ "$output" == *"by panel"* ]]
  [[ "$output" == *"cc cod agy"* ]]
  [[ "$output" == *"AGREEMENT-COLLAPSE"* ]]
}
