#!/usr/bin/env bats
# Skill validators must be able to fail (the #996 doctrine, applied to
# skills/*/scripts/*.sh). Under `set -euo pipefail` a `! command` line is a
# silent no-op: errexit never fires for `!`-inverted pipelines, so a
# forbidden-phrase guard written that way can never fail the script. Seven
# validators shipped in that state (rpi, plan, implement, learn, ms,
# scaffold, security). Learn and Scaffold have since been folded into current
# task owners, MS was withdrawn, and the rpi, plan and implement prose
# validators were retired as wording pins (age-pv2j.37). It pins the surviving
# guards (memory, security, research, premortem) statically and behaviorally.

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

@test "memory validator fails when source reading is assigned to the wrong command owner" {
  # Memory absorbs learning and owns this live source-access boundary.
  # The retired Learn prose validator is no longer an executable contract.
  seeded_validator_must_fail memory 'ao provenance read-source'
}

# Scaffold's prose-only AUTO-REDO ban retired with its separate skill contract.
# Implementation owns authorized scaffolding and repairs; do not reintroduce
# the old no-repair policy as a liveness test of the surviving owner.

@test "security validator fails when effects are declared empty" {
  seeded_validator_must_fail security '  effects: []'
}

@test "security validator fails on seeded forbidden token" {
  seeded_validator_must_fail security 'AUTO-REDO'
}

@test "research retired-lifecycle scan fails closed without ripgrep" {
  local copy="$BATS_TEST_TMPDIR/research" bin="$BATS_TEST_TMPDIR/no-rg-bin" t
  mkdir -p "$copy" "$bin"
  cp -R "$REPO_ROOT/skills/research/." "$copy/"
  for t in bash grep python3 cat dirname; do ln -s "$(command -v "$t")" "$bin/$t"; done
  run env PATH="$bin" "$bin/bash" "$copy/scripts/validate.sh"
  [ "$status" -eq 0 ]
  printf '\nRun ao lookup --query x first.\n' >> "$copy/references/status.md"
  run env PATH="$bin" "$bin/bash" "$copy/scripts/validate.sh"
  [ "$status" -eq 1 ]
}

@test "premortem validator fails when the advisory review claims lifecycle authority" {
  # Premortem only reports findings on a plan; it never lands, commits or
  # redoes work. Before this test nothing in CI ran its validator.
  seeded_validator_must_fail premortem 'Then run ao land on the reviewed plan.'
}

@test "validators still pass on the live skill sources" {
  for slug in memory security premortem research test; do
    run bash "$REPO_ROOT/skills/$slug/scripts/validate.sh"
    [ "$status" -eq 0 ]
  done
}
