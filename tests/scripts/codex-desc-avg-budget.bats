#!/usr/bin/env bats
# ag-vzbt: the codex-description catalog budget must be a per-skill AVERAGE (scales
# with skill count) instead of a hard aggregate that walls off the Nth+ skill.
# Fixture-driven via BUDGET_REPO_ROOT. We assert on the codex-catalog output line
# (robust to unrelated checks in the gate).

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
  mk a "short terse codex description here"   # ~34 chars
  mk b "another short terse codex description"  # ~36 chars
  run env BUDGET_REPO_ROOT="$FIX" bash "$GATE"
  echo "$output"
  [[ "$output" == *"skills-codex description catalog"* ]]
  echo "$output" | grep "skills-codex description catalog" | grep -q "PASS"
}

@test "codex catalog FAILS when average description length is over budget" {
  # Each ~120 chars → avg ~120 >> 45 per-skill-avg cap, but still < 180 per-skill hard cap.
  long="this is a deliberately verbose codex description that pushes the per skill average well over the configured budget ceiling"
  mk a "$long"
  mk b "$long"
  run env BUDGET_REPO_ROOT="$FIX" bash "$GATE"
  echo "$output"
  echo "$output" | grep "skills-codex description catalog" | grep -q "FAIL"
}

@test "budget scales: 100 short-desc skills pass even though total > old 2800 wall" {
  # 100 x ~38 chars = ~3800 total (> the old 2800 hard aggregate) but avg ~38 < 45.
  # Passes ONLY under the per-skill-average rule — proves the wall is gone.
  for i in $(seq 1 100); do mk "skill$i" "terse codex description number $i here"; done
  run env BUDGET_REPO_ROOT="$FIX" bash "$GATE"
  echo "$output"
  echo "$output" | grep "skills-codex description catalog" | grep -q "PASS"
}
