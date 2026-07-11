#!/usr/bin/env bats
# pawl bench/unbench (F7-followup, age-pawl-intent-zhndq.18): benching is a persisted, visible
# state — `ao pawl bench <fam>` writes .agents/pawl/config.json, resolution reads it (source:
# config), and the PAWL_BENCHED_FAMILIES env still wins as a one-shot override (source: env).

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/pawl.sh"
  TMP="$(mktemp -d)"; REPO="$TMP/repo"; mkdir -p "$REPO"; cd "$REPO"; git init -q
  CFG="$REPO/.agents/pawl/config.json"
}
teardown() { rm -rf "$TMP"; }

@test "bench <fam> persists to config.json; unbench removes it" {
  run env -u PAWL_BENCHED_FAMILIES bash "$SCRIPT" bench cod
  [ "$status" -eq 0 ]
  [ -f "$CFG" ]
  grep -q '"cod"' "$CFG"
  run env -u PAWL_BENCHED_FAMILIES bash "$SCRIPT" unbench cod
  [ "$status" -eq 0 ]
  # cod removed (array now empty), not still present
  ! grep -q '"cod"' "$CFG"
}

@test "bench list reads the persisted config (source: config)" {
  env -u PAWL_BENCHED_FAMILIES bash "$SCRIPT" bench agy >/dev/null 2>&1
  run env -u PAWL_BENCHED_FAMILIES bash "$SCRIPT" bench
  [[ "$output" == *"[agy]"* ]]
  [[ "$output" == *"source: config"* ]]
}

@test "env PAWL_BENCHED_FAMILIES wins over config (source: env)" {
  env -u PAWL_BENCHED_FAMILIES bash "$SCRIPT" bench cod >/dev/null 2>&1   # config says cod
  run env PAWL_BENCHED_FAMILIES="agy cc" bash "$SCRIPT" bench
  [[ "$output" == *"[agy cc]"* ]]      # env value, not the config's cod
  [[ "$output" == *"source: env"* ]]
}

@test "bench rejects an unknown family (exit non-zero, no config write)" {
  run env -u PAWL_BENCHED_FAMILIES bash "$SCRIPT" bench bogus
  [ "$status" -ne 0 ]
  [ ! -f "$CFG" ]
}

@test "unbench with no arg -> clean usage, exit 2 (no bash trace)" {
  run env -u PAWL_BENCHED_FAMILIES bash "$SCRIPT" unbench
  [ "$status" -eq 2 ]
  [[ "$output" == *"usage: ao pawl unbench"* ]]
  [[ "$output" != *"line "* ]]
}
