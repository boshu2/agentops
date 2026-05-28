#!/usr/bin/env bats
# Regression tests for autodev skill positioning (soc-ozoqh, scenario 1).
#
# The canonical model in skills/domain/references/autodev.md:9,13 prescribes that
# autodev reads as the *config/intent layer that drives the loop*, NEVER as
# "bounded autonomous dev loops" (that phrasing implies a loop and was the source
# of the original sprawl). These tests pin the autodev SKILL.md to that model:
# the description leads with the PROGRAM.md/AUTODEV.md contract framing, names
# autodev as the config layer the Evolve/Factory drivers consume, and the banned
# phrase appears nowhere in the skill.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SKILL="$REPO_ROOT/skills/autodev/SKILL.md"
}

# Extract the single-line frontmatter `description:` value.
description_line() {
  grep -m1 '^description:' "$SKILL"
}

@test "skill exists with autodev frontmatter" {
  [ -f "$SKILL" ]
  grep -q '^name: autodev$' "$SKILL"
}

@test "description drops the banned 'bounded autonomous dev loops' phrasing" {
  run description_line
  [ "$status" -eq 0 ]
  [[ "$output" != *"bounded autonomous dev loops"* ]]
}

@test "description leads with the PROGRAM.md/AUTODEV.md contract framing" {
  run description_line
  [ "$status" -eq 0 ]
  [[ "$output" == *"PROGRAM.md"* ]]
  [[ "$output" == *"AUTODEV.md"* ]]
  [[ "$output" == *"contract"* ]]
}

@test "the banned phrase appears nowhere in the skill body" {
  run grep -ci 'bounded autonomous dev loops' "$SKILL"
  [ "$output" -eq 0 ]
}

@test "skill names autodev as config that drives the loop, not a loop it runs" {
  # Config-layer framing, per skills/domain/references/autodev.md.
  grep -qi 'config' "$SKILL"
  grep -qi 'drives the loop' "$SKILL"
  # Names both drivers that consume the contract.
  grep -q 'Evolve' "$SKILL"
  grep -q 'Factory' "$SKILL"
  # No longer claims autodev itself runs the loop unattended.
  ! grep -qi 'runs the loop unattended' "$SKILL"
}
