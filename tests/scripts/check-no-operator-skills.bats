#!/usr/bin/env bats
# Regression coverage for scripts/check-no-operator-skills.sh
# (bead age-focus-membrane-bookkeeper-m1wg.11): the published product catalog
# must never expose operator/personal-identity skills.

setup() {
  SCRIPT="$BATS_TEST_DIRNAME/../../scripts/check-no-operator-skills.sh"
  ROOT="$(mktemp -d)"
}

teardown() {
  rm -rf "$ROOT"
}

@test "clean catalog (product skills only) passes" {
  mkdir -p "$ROOT/skills/research" "$ROOT/skills/validate"
  run bash "$SCRIPT" "$ROOT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"product skills only"* ]]
}

@test "substrate skills (ntm/atm/swarm) are NOT flagged" {
  mkdir -p "$ROOT/skills/ntm" "$ROOT/skills/using-atm" "$ROOT/skills/swarm"
  run bash "$SCRIPT" "$ROOT"
  [ "$status" -eq 0 ]
}

@test "an operator-personal skill dir (athena) fails fail-closed" {
  mkdir -p "$ROOT/skills/athena"
  run bash "$SCRIPT" "$ROOT"
  [ "$status" -eq 1 ]
  [[ "$output" == *"athena"* ]]
}

@test "a personal-identity twin (skills-codex/wealth-mentor) fails" {
  mkdir -p "$ROOT/skills/research" "$ROOT/skills-codex/wealth-mentor"
  run bash "$SCRIPT" "$ROOT"
  [ "$status" -eq 1 ]
  [[ "$output" == *"wealth-mentor"* ]]
}

@test "a published-catalog reference (registry.json) to a denied slug fails" {
  mkdir -p "$ROOT/skills/research"
  printf '{ "skills": [ { "name": "bo-voice" } ] }\n' > "$ROOT/registry.json"
  run bash "$SCRIPT" "$ROOT"
  [ "$status" -eq 1 ]
  [[ "$output" == *"bo-voice"* ]]
}

@test "--self-test passes" {
  run bash "$SCRIPT" --self-test
  [ "$status" -eq 0 ]
  [[ "$output" == *"self-test OK"* ]]
}
