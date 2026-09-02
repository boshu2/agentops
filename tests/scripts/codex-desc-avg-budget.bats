#!/usr/bin/env bats
# ag-vzbt: the codex-description catalog budget must be a per-skill AVERAGE (scales
# with skill count) instead of a hard aggregate that walls off the Nth+ skill.
# Fixture-driven via BUDGET_REPO_ROOT. We assert on the codex-catalog output line
# (robust to unrelated checks in the gate).
#
# The limit under test is CODEX_DESC_AVG_FAIL_CHARS in
# tests/skills/test-token-budgets.sh, currently 96 (the Claude catalog's prose
# average, measured by that file's own extraction). The fixtures below are sized
# against 96, not against a number of their own: a fixture that stops
# discriminating when the limit moves is a fixture that must be resized here,
# deliberately, with the margin restated.

setup() {
  GATE="$BATS_TEST_DIRNAME/../../tests/skills/test-token-budgets.sh"
  FIX="$(mktemp -d)"
  mkdir -p "$FIX/skills" "$FIX/skills-codex"
  mk() { # mk <name> <description>
    mkdir -p "$FIX/skills-codex/$1"
    printf -- '---\nname: %s\ndescription: %s\n---\n# %s\n' "$1" "$2" "$1" > "$FIX/skills-codex/$1/SKILL.md"
  }
  export -f mk
}

teardown() { rm -rf "$FIX"; }

@test "codex catalog PASSES when average description length is under budget" {
  mk a "short terse codex description here"     # 34 chars
  mk b "another short terse codex description"  # 37 chars — avg 35, well under 96
  run env BUDGET_REPO_ROOT="$FIX" bash "$GATE"
  echo "$output"
  [[ "$output" == *"skills-codex description catalog"* ]]
  echo "$output" | grep "skills-codex description catalog" | grep -q "PASS"
}

@test "codex catalog FAILS when average description length is over budget" {
  # Each 122 chars → avg 122 > the 96 per-skill-avg limit (26 chars of margin),
  # but still under the 180-char per-skill hard cap, so this isolates the AVERAGE
  # rule from the per-entry rule.
  long="this is a deliberately verbose codex description that pushes the per skill average well over the configured budget ceiling"
  mk a "$long"
  mk b "$long"
  run env BUDGET_REPO_ROOT="$FIX" bash "$GATE"
  echo "$output"
  echo "$output" | grep "skills-codex description catalog" | grep -q "FAIL"
}

@test "budget scales: 100 short-desc skills pass even though total > old 2800 wall" {
  # 100 x 37 chars = ~3700 total (> the old 2800 hard aggregate) but avg 37 < 96.
  # Passes ONLY under the per-skill-average rule — proves the wall is gone.
  for i in $(seq 1 100); do mk "skill$i" "terse codex description number $i here"; done
  run env BUDGET_REPO_ROOT="$FIX" bash "$GATE"
  echo "$output"
  echo "$output" | grep "skills-codex description catalog" | grep -q "PASS"
}
