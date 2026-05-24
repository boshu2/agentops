#!/usr/bin/env bats
# Regression tests for the using-gc skill (ag-p4p).
#
# using-gc GUIDES agents on the gc (Gas City) CLI the way the bd protocol guides
# them. These tests assert the skill stays accurate to the reference pack: it
# frames gc as a guided dependency (not an ao wrapper), names the gc primitives,
# documents the mayor-driven dispatch loop, is honest about the order-auto-
# dispatch gap (soc-5jwah), and links every reference doc. The @test names here
# are referenced by @covered-by: tags in references/using-gc.feature — keep them
# in sync.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SKILL="$REPO_ROOT/skills/using-gc/SKILL.md"
  REFDIR="$REPO_ROOT/skills/using-gc/references"
}

@test "skill exists with meta frontmatter" {
  [ -f "$SKILL" ]
  run head -1 "$SKILL"
  [ "$status" -eq 0 ]
  [ "$output" = "---" ]
  grep -q '^name: using-gc$' "$SKILL"
  grep -q '^  tier: meta$' "$SKILL"
  grep -q '^  internal: true$' "$SKILL"
  # gc is a GUIDED dependency, NOT wrapped by ao.
  grep -qi 'ao.*does NOT wrap' "$SKILL"
}

@test "skill names the core gc primitives" {
  for primitive in City Rig Pack Agent Order Formula Mayor Refinery; do
    grep -q "$primitive" "$SKILL"
  done
}

@test "skill documents the mayor-driven dispatch loop" {
  grep -qi 'mayor-driven' "$SKILL"
  grep -q 'gc start' "$SKILL"
  grep -q 'gc rig add' "$SKILL"
  grep -q 'gc sling' "$SKILL"
  grep -q 'bd ready' "$SKILL"
  grep -q 'ao rpi' "$SKILL"
}

@test "skill is honest about the order-auto-dispatch gap" {
  grep -q 'soc-5jwah' "$SKILL"
  grep -qi 'order-auto-dispatch' "$SKILL"
  grep -qi 'upstream' "$SKILL"
  # The honest framing: dispatch is mayor-driven today, not turnkey order-auto.
  grep -qi 'mayor-driven' "$SKILL"
}

@test "skill links every reference doc" {
  for ref in "$REFDIR"/*; do
    base="$(basename "$ref")"
    grep -qF "references/$base" "$SKILL"
  done
}
