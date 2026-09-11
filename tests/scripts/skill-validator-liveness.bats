#!/usr/bin/env bats
# Skill validators must be able to fail (the #996 doctrine, applied to
# skills/*/scripts/*.sh). Under `set -euo pipefail` a `! command` line is a
# silent no-op: errexit never fires for `!`-inverted pipelines, so a
# forbidden-phrase guard written that way can never fail the script. Seven
# validators shipped in that state (rpi, plan, implement, learn, ms,
# scaffold, security). Learn and Scaffold have since been folded into current
# task owners. This file pins the surviving guard class both statically and
# behaviorally, using each owner's actual restrictions.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
}

@test "no inert !-negation guards in skill validators" {
  run grep -rEn '^[[:space:]]*![[:space:]]' "$REPO_ROOT"/skills/*/scripts/*.sh
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
  # The relocated copy must pass unseeded, or a failure below would prove
  # relocation breakage rather than guard liveness.
  run bash "$copy/scripts/validate.sh"
  [ "$status" -eq 0 ]
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

@test "memory validator fails when source reading is assigned to the wrong command owner" {
  # Memory absorbs learning and owns this live source-access boundary.
  # The retired Learn prose validator is no longer an executable contract.
  seeded_validator_must_fail memory 'ao provenance read-source'
}

@test "ms validator fails on seeded forbidden token" {
  seeded_validator_must_fail ms 'AUTO-REDO'
}

# Scaffold's prose-only AUTO-REDO ban retired with its separate skill contract.
# Implementation owns authorized scaffolding and repairs; do not reintroduce
# the old no-repair policy as a liveness test of the surviving owner.

@test "security validator fails on seeded forbidden token" {
  seeded_validator_must_fail security 'AUTO-REDO'
}

@test "validators still pass on the live skill sources" {
  for slug in rpi plan implement memory ms security; do
    run bash "$REPO_ROOT/skills/$slug/scripts/validate.sh"
    [ "$status" -eq 0 ]
  done
}
