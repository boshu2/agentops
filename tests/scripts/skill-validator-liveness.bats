#!/usr/bin/env bats
# Skill validators must be able to fail (the #996 doctrine, applied to
# skills/*/scripts/*.sh). Under `set -euo pipefail` a `! command` line is a
# silent no-op: errexit never fires for `!`-inverted pipelines, so a
# forbidden-phrase guard written that way can never fail the script. Seven
# validators shipped in that state (rpi, plan, implement, learn, ms,
# scaffold, security). This file pins the repaired class both statically
# and behaviorally.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
}

@test "no inert !-negation guards in skill validators" {
  run grep -rn '^[[:space:]]*! ' "$REPO_ROOT"/skills/*/scripts/*.sh
  [ "$status" -ne 0 ]
}

# Behavioral liveness: seeding the forbidden token into a copy of the skill
# must make its validator exit nonzero. A validator that passes here is the
# original defect regrown.
seeded_validator_must_fail() {
  local slug="$1" token="$2"
  local copy="$BATS_TEST_TMPDIR/$slug"
  mkdir -p "$copy"
  cp -R "$REPO_ROOT/skills/$slug/." "$copy/"
  printf '\n%s\n' "$token" >> "$copy/SKILL.md"
  run bash "$copy/scripts/validate.sh"
  [ "$status" -ne 0 ]
}

@test "rpi validator fails on seeded forbidden token" {
  seeded_validator_must_fail rpi 'plan_packet_digest'
}

@test "plan validator fails on seeded forbidden token" {
  seeded_validator_must_fail plan 'plan-packet.v1'
}

@test "implement validator fails on seeded forbidden token" {
  seeded_validator_must_fail implement 'candidate-packet.v1'
}

@test "learn validator fails on seeded forbidden token" {
  seeded_validator_must_fail learn 'emit a lifecycle receipt'
}

@test "ms validator fails on seeded forbidden token" {
  seeded_validator_must_fail ms 'AUTO-REDO'
}

@test "scaffold validator fails on seeded forbidden token" {
  seeded_validator_must_fail scaffold 'AUTO-REDO'
}

@test "security validator fails on seeded forbidden token" {
  seeded_validator_must_fail security 'AUTO-REDO'
}

@test "validators still pass on the live skill sources" {
  for slug in rpi plan implement learn ms scaffold security; do
    run bash "$REPO_ROOT/skills/$slug/scripts/validate.sh"
    [ "$status" -eq 0 ]
  done
}
