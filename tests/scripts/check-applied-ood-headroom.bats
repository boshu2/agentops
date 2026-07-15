#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/check-applied-ood-headroom.sh"
  TMP="$(mktemp -d)"
  SCENARIOS="$TMP/scenarios"
  OUT="$TMP/out"
  mkdir -p "$SCENARIOS" "$TMP/bin"
  AO_LOG="$TMP/ao.log"
  export AO_LOG
}

teardown() {
  rm -rf "$TMP"
}

write_scenario() {
  local id="$1"
  cat > "$SCENARIOS/$id.json" <<JSON
{"schema_version":1,"id":"$id","version":1,"date":"2026-06-17","goal":"apply doctrine","narrative":"n","expected_outcome":"e","acceptance_vectors":[{"dimension":"applies-doctrine","threshold":0.8}],"satisfaction_threshold":0.8,"source":"human","status":"active"}
JSON
}

write_fake_ao() {
  local status="${1:-0}"
  cat > "$TMP/bin/ao" <<FAKE
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "\$*" >> "\$AO_LOG"
out=""
prev=""
for arg in "\$@"; do
  if [[ "\$prev" == "--output" ]]; then
    out="\$arg"
  fi
  prev="\$arg"
done
if [[ -n "\$out" ]]; then
  mkdir -p "\$(dirname "\$out")"
  printf '{"control_only":true,"gate":{"pass":%s}}\n' "$([[ "$status" == 0 ]] && printf true || printf false)" > "\$out"
fi
exit $status
FAKE
  chmod +x "$TMP/bin/ao"
}

@test "applied-OOD headroom hook runs scenario-ab control-only for every scenario" {
  write_scenario s-2026-06-17-001
  write_scenario s-2026-06-17-002
  write_fake_ao 0

  run env AO_BIN="$TMP/bin/ao" "$SCRIPT" --scenario-dir "$SCENARIOS" --out-dir "$OUT"

  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS applied-OOD headroom"* ]]
  [ "$(wc -l < "$AO_LOG" | tr -d ' ')" -eq 2 ]
  grep -Fq "eval scenario-ab --control-only --scenario $SCENARIOS/s-2026-06-17-001.json --output $OUT/s-2026-06-17-001.scorecard.json" "$AO_LOG"
  grep -Fq "eval scenario-ab --control-only --scenario $SCENARIOS/s-2026-06-17-002.json --output $OUT/s-2026-06-17-002.scorecard.json" "$AO_LOG"
  [ -f "$OUT/s-2026-06-17-001.scorecard.json" ]
  [ -f "$OUT/s-2026-06-17-002.scorecard.json" ]
}

@test "applied-OOD headroom hook fails closed when a control-only scenario fails" {
  write_scenario s-2026-06-17-001
  write_fake_ao 1

  run env AO_BIN="$TMP/bin/ao" "$SCRIPT" --scenario-dir "$SCENARIOS" --out-dir "$OUT"

  [ "$status" -eq 1 ]
  [[ "$output" == *"FAIL applied-OOD headroom"* ]]
  grep -Fq "eval scenario-ab --control-only" "$AO_LOG"
}

@test "validate workflow still mentions applied-OOD headroom (path filter / coverage)" {
  run grep -F "scripts/check-applied-ood-headroom.sh" "$REPO_ROOT/.github/workflows/validate.yml"
  [ "$status" -eq 0 ]

  [ ! -e "$REPO_ROOT/scripts/hooks/pre-push.local" ]
}
