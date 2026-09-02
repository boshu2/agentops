#!/usr/bin/env bats
# ag-vzbt: the codex-description catalog budget must be a per-skill AVERAGE (scales
# with skill count) instead of a hard aggregate that walls off the Nth+ skill.
# Fixture-driven via BUDGET_REPO_ROOT. We assert on the codex-catalog output line
# (robust to unrelated checks in the gate).
#
# The rule under test is LIVE, not a stored constant: the Codex catalog's prose
# average may not exceed the CLAUDE catalog's prose average, both measured by
# test-token-budgets.sh's own extraction. Every fixture below therefore builds
# BOTH sides — a skills/ set and a skills-codex/ set — and the discriminating
# variable is the relationship between them, not a magic number that goes stale.

setup() {
  GATE="$BATS_TEST_DIRNAME/../../tests/skills/test-token-budgets.sh"
  FIX="$(mktemp -d)"
  mkdir -p "$FIX/skills" "$FIX/skills-codex"
}

teardown() { rm -rf "$FIX"; }

# mk <root> <name> <description>
mk() {
  mkdir -p "$FIX/$1/$2"
  printf -- '---\nname: %s\ndescription: %s\n---\n# %s\n' "$2" "$3" "$2" > "$FIX/$1/$2/SKILL.md"
}

# chars <n> — a description of exactly n characters, for exact-average fixtures.
chars() { printf 'x%.0s' $(seq 1 "$1"); }

catalog_line() {
  run env BUDGET_REPO_ROOT="$FIX" bash "$GATE"
  echo "$output"
  echo "$output" | grep 'skills-codex description catalog'
}

@test "codex catalog PASSES when its average equals the Claude average" {
  # The real-world case: the twin description IS the projection of the source,
  # so the two averages match exactly. Equal must pass — only GREATER fails.
  mk skills a "short terse codex description here"
  mk skills b "another short terse codex description"
  mk skills-codex a "short terse codex description here"
  mk skills-codex b "another short terse codex description"
  catalog_line | grep -q "PASS"
}

@test "codex catalog PASSES when its average is below the Claude average" {
  mk skills a "$(chars 120)"
  mk skills b "$(chars 120)"
  mk skills-codex a "$(chars 60)"
  mk skills-codex b "$(chars 60)"
  catalog_line | grep -q "PASS"
}

@test "codex catalog FAILS when its average exceeds the Claude average" {
  # Claude avg 40, Codex avg 122 — the projection got verbose relative to its
  # own source. Both stay under the 180-char per-entry cap, so this isolates
  # the AVERAGE rule from the per-entry rule.
  mk skills a "$(chars 40)"
  mk skills b "$(chars 40)"
  mk skills-codex a "$(chars 122)"
  mk skills-codex b "$(chars 122)"
  catalog_line | grep -q "FAIL"
}

@test "codex catalog FAILS on a fractional excess that integer flooring would hide" {
  # The exact regression the flooring bug allowed through: Codex true average
  # 96.9, Claude true average 96.0. Both floor to 96, so a floored comparison
  # reported PASS. The cross-multiplied comparison (969*1 > 96*10) catches it.
  mk skills only "$(chars 96)"
  for i in $(seq 1 9); do mk skills-codex "c$i" "$(chars 97)"; done
  mk skills-codex c10 "$(chars 96)"
  catalog_line | grep -q "FAIL"
}

@test "budget scales: 100 skills pass even though total > old 2800 wall" {
  # 100 x 37 chars = 3700 total on each side (> the old 2800 hard aggregate),
  # equal averages. Passes ONLY under the per-skill-average rule — proves the
  # wall is gone.
  for i in $(seq 1 100); do
    mk skills "skill$i" "terse codex description number $i here"
    mk skills-codex "skill$i" "terse codex description number $i here"
  done
  catalog_line | grep -q "PASS"
}

@test "codex catalog FAILS when no Claude catalog exists to bound it" {
  # An empty skills/ side is not a free pass: with nothing to compare against,
  # the bound is unknown, and unknown is not PASS.
  mk skills-codex a "short terse codex description here"
  catalog_line | grep -q "FAIL"
}
