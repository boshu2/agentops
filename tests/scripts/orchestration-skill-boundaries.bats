#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  FIXTURE_ROOT="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$FIXTURE_ROOT/scripts"
  cp \
    "$REPO_ROOT/scripts/check-orchestration-skill-boundaries.sh" \
    "$FIXTURE_ROOT/scripts/check-orchestration-skill-boundaries.sh"

  for slug in \
    ntm agent-mail agent-native automation-shape-routing swarm using-gc; do
    mkdir -p "$FIXTURE_ROOT/skills/$slug"
    : >"$FIXTURE_ROOT/skills/$slug/SKILL.md"
  done
  printf '%s\n' \
    'A single local agent pays no factory coordination cost.' \
    >"$FIXTURE_ROOT/skills/agent-native/SKILL.md"
  printf '%s\n' \
    'External NTM documentation remains authoritative.' \
    >"$FIXTURE_ROOT/skills/ntm/SKILL.md"
  printf '%s\n' \
    'Use the self-describing `am` CLI.' \
    >"$FIXTURE_ROOT/skills/agent-mail/SKILL.md"
}

@test "current orchestration boundaries pass without retired CLI adapters" {
  run bash "$FIXTURE_ROOT/scripts/check-orchestration-skill-boundaries.sh"

  [ "$status" -eq 0 ]
  [[ "$output" == *"orchestration skill boundaries: PASS"* ]]
}

@test "ATM-era terminology is rejected for an active orchestration skill" {
  printf '%s\n' \
    'Route this request through ATM.' \
    >>"$FIXTURE_ROOT/skills/swarm/SKILL.md"

  run bash "$FIXTURE_ROOT/scripts/check-orchestration-skill-boundaries.sh"

  [ "$status" -ne 0 ]
  [[ "$output" == *"ATM-era naming remains"* ]]
}
