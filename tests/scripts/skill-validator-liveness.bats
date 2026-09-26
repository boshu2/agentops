#!/usr/bin/env bats
# Skill validators must be able to fail (the #996 doctrine, applied to
# skills/*/scripts/*.sh). Under `set -euo pipefail` a `! command` line is a
# silent no-op: errexit never fires for `!`-inverted pipelines, so a
# forbidden-phrase guard written that way can never fail the script. Seven
# validators shipped in that state (rpi, plan, implement, learn, ms,
# scaffold, security). Learn and Scaffold have since been folded into current
# task owners, and MS was withdrawn. This file pins the surviving guard class both statically and
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

@test "plan validator rejects the former mandatory control in its reference" {
  local copy="$BATS_TEST_TMPDIR/plan"
  mkdir -p "$copy"
  cp -R "$REPO_ROOT/skills/plan/." "$copy/"
  run bash "$copy/scripts/validate.sh"
  [ "$status" -eq 0 ]
  printf '\nEvery plan needs a ground truth, control experiment and deviation ledger.\n' >> "$copy/references/ground-truth-routing.md"
  run bash "$copy/scripts/validate.sh"
  [ "$status" -ne 0 ]
  [[ "$output" == *"mandatory control or ledger"* ]]
}

@test "plan validator permits native recovery facts and rejects their blanket ban" {
  local copy="$BATS_TEST_TMPDIR/plan"
  mkdir -p "$copy"
  cp -R "$REPO_ROOT/skills/plan/." "$copy/"
  printf '\nNative handoff: owner and next action are references to the caller tracker.\n' >> "$copy/SKILL.md"
  run bash "$copy/scripts/validate.sh"
  [ "$status" -eq 0 ]
  printf '\nThen it contains no owner, ready, claim, priority, attempt, wave, queue, lease, admission, next action\n' >> "$copy/references/plan.feature"
  run bash "$copy/scripts/validate.sh"
  [ "$status" -ne 0 ]
  [[ "$output" == *"forbid factual native handoff recovery"* ]]
}

@test "plan validator refuses an unshipped selective method" {
  local copy="$BATS_TEST_TMPDIR/plan"
  mkdir -p "$copy"
  cp -R "$REPO_ROOT/skills/plan/." "$copy/"
  run bash "$copy/scripts/validate.sh"
  [ "$status" -eq 0 ]
  mv "$copy/references/challenge.md" "$copy/references/challenge.missing"
  run bash "$copy/scripts/validate.sh"
  [ "$status" -ne 0 ]
}

@test "implement validator fails on seeded forbidden token" {
  seeded_validator_must_fail implement 'candidate-packet.v1'
}

@test "memory validator fails when source reading is assigned to the wrong command owner" {
  # Memory absorbs learning and owns this live source-access boundary.
  # The retired Learn prose validator is no longer an executable contract.
  seeded_validator_must_fail memory 'ao provenance read-source'
}

# Scaffold's prose-only AUTO-REDO ban retired with its separate skill contract.
# Implementation owns authorized scaffolding and repairs; do not reintroduce
# the old no-repair policy as a liveness test of the surviving owner.

@test "security validator fails on seeded forbidden token" {
  seeded_validator_must_fail security 'AUTO-REDO'
}

@test "validators still pass on the live skill sources" {
  for slug in rpi plan implement memory security; do
    run bash "$REPO_ROOT/skills/$slug/scripts/validate.sh"
    [ "$status" -eq 0 ]
  done
}
