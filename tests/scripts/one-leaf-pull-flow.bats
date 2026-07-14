#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  VALIDATOR="$REPO_ROOT/skills/rpi/scripts/validate-execution-packet.py"
  PACKET="$BATS_TEST_TMPDIR/execution-packet.json"
  mkdir -p "$REPO_ROOT/.agents/rpi"
  DISCOVERY_ARTIFACT="$REPO_ROOT/.agents/rpi/s4-discovery-$BATS_TEST_NUMBER-$$.md"
  printf 'discovery evidence\n' >"$DISCOVERY_ARTIFACT"
}

teardown() {
  rm -f "$DISCOVERY_ARTIFACT"
}

write_prospective_packet() {
  jq -n --arg artifact ".agents/rpi/$(basename "$DISCOVERY_ARTIFACT")" '{
    schema_version: 3,
    packet_state: "prospective",
    objective: "pull one bounded leaf",
    skills_loaded: [
      {name: "rpi", reason: "orchestrator"},
      {name: "discovery", reason: "phase-1"}
    ],
    phase_receipts: [
      {phase: "discovery", skill: "discovery", status: "DONE", artifact: $artifact},
      {phase: "crank", skill: "crank", status: "pending"},
      {phase: "validate", skill: "validate", status: "not_checked"},
      {phase: "learn", skill: "learn", status: "not_checked"}
    ]
  }' >"$PACKET"
}

@test "goal parents remain aggregate demand while one writer owns one active leaf" {
  run rg -n 'Goal and epic parents are aggregate demand, never writer WIP' \
    "$REPO_ROOT/skills/plan/SKILL.md" \
    "$REPO_ROOT/skills/behavior-first-planning/SKILL.md"
  [ "$status" -eq 0 ]
  [ "$(grep -c 'Goal and epic parents are aggregate demand, never writer WIP' <<<"$output")" -eq 2 ]

  run rg -n 'one active leaf per writer' \
    "$REPO_ROOT/skills/plan/SKILL.md" \
    "$REPO_ROOT/skills/discovery/SKILL.md" \
    "$REPO_ROOT/skills/rpi/SKILL.md"
  [ "$status" -eq 0 ]
  [ "$(grep -c 'one active leaf per writer' <<<"$output")" -eq 3 ]
}

@test "Discovery can hand off an honest prospective packet with pending phases" {
  write_prospective_packet

  run python3 "$VALIDATOR" "$PACKET"

  [ "$status" -eq 0 ]
  [[ "$output" == *"valid prospective execution packet"* ]]
}

@test "a terminal packet cannot reuse pending or not_checked phase state" {
  write_prospective_packet
  jq '
    .packet_state = "terminal"
    | .skills_loaded += [
        {name: "crank", reason: "phase-2"},
        {name: "validate", reason: "phase-3"},
        {name: "learn", reason: "phase-4"}
      ]
  ' "$PACKET" >"$PACKET.tmp"
  mv "$PACKET.tmp" "$PACKET"

  run python3 "$VALIDATOR" "$PACKET"

  [ "$status" -eq 1 ]
  [[ "$output" == *"terminal phase_receipts"* ]]
}

@test "a prospective packet cannot claim an unrun phase skill as loaded" {
  write_prospective_packet
  jq '.skills_loaded += [{name: "crank", reason: "future-phase"}]' \
    "$PACKET" >"$PACKET.tmp"
  mv "$PACKET.tmp" "$PACKET"

  run python3 "$VALIDATOR" "$PACKET"

  [ "$status" -eq 1 ]
  [[ "$output" == *"prospective skills_loaded must omit unrun phase skill: crank"* ]]
}

@test "a prospective packet cannot fabricate downstream success" {
  write_prospective_packet
  jq '(.phase_receipts[] | select(.phase != "discovery") | .status) = "DONE"' \
    "$PACKET" >"$PACKET.tmp"
  mv "$PACKET.tmp" "$PACKET"

  run python3 "$VALIDATOR" "$PACKET"

  [ "$status" -eq 1 ]
  [[ "$output" == *"prospective phase_receipts"* ]]
}

@test "each admitted wave with remaining work requires one bounded Premortem" {
  run rg -n 'Every admitted Crank wave with remaining work must end with exactly one bounded Premortem' \
    "$REPO_ROOT/skills/rpi/SKILL.md" \
    "$REPO_ROOT/skills/premortem/SKILL.md"
  [ "$status" -eq 0 ]
  [ "$(grep -c 'Every admitted Crank wave with remaining work must end with exactly one bounded Premortem' <<<"$output")" -eq 2 ]
}

@test "a second distinct repair need routes to REPLAN without a Discovery-local controller" {
  run rg -n 'second distinct repair need.*REPLAN' \
    "$REPO_ROOT/skills/discovery/SKILL.md" \
    "$REPO_ROOT/skills/rpi/SKILL.md"
  [ "$status" -eq 0 ]
  [ "$(grep -c 'second distinct repair need.*REPLAN' <<<"$output")" -eq 2 ]

  [ ! -e "$REPO_ROOT/skills/discovery/scripts/mvp-helper-state.sh" ]
  [ ! -e "$REPO_ROOT/skills/discovery/scripts/validate-contract-fixtures.sh" ]
  run rg -n 'up to 3 total attempts|discovery_mvp_helper|mvp-helper-state' \
    "$REPO_ROOT/skills/discovery"
  [ "$status" -eq 1 ]
}

@test "candidate-attributed Gemini runtime consumers match source policy" {
  for skill in plan discovery premortem; do
    run cmp -s \
      "$REPO_ROOT/skills/$skill/SKILL.md" \
      "$REPO_ROOT/images/gemini/skills/$skill/SKILL.md"
    [ "$status" -eq 0 ]
  done

  run rg -n 'three MVP premortem failures|ordinary MVP breaker|bounded helper' \
    "$REPO_ROOT/images/gemini/skills/discovery/SKILL.md"
  [ "$status" -eq 1 ]
}
