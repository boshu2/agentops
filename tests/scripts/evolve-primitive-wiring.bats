#!/usr/bin/env bats
# Consumer-wiring proof for the soc-g2qd /evolve CLI primitives.
#
# The soc-g2qd epic shipped six primitives the skill never called (the failure
# this guards against). These tests assert that skills/evolve/SKILL.md actually
# invokes the three primitives that were wired (write-stop-marker, next-work,
# blocked) at their real decision sites. If a future edit drops a wire, the
# corresponding test fails — the orphaned-primitive regression guard.
#
# Behavior of each primitive is covered by its own Go tests
# (cli/cmd/ao/evolve_*_test.go); these tests are wiring-only.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SKILL="$REPO_ROOT/skills/evolve/SKILL.md"
}

@test "SKILL.md Step 3 invokes 'ao evolve next-work' for work selection" {
  run bash -c "sed -n '/### Step 3: Select Work/,/### Step 4/p' '$SKILL' | grep -F 'ao evolve next-work'"
  [ "$status" -eq 0 ]
}

@test "SKILL.md routes the stagnation marker write through 'ao evolve write-stop-marker'" {
  run grep -F 'ao evolve write-stop-marker --marker dormant' "$SKILL"
  [ "$status" -eq 0 ]
}

@test "SKILL.md logs a typed blocked event via 'ao evolve blocked' instead of halting" {
  run grep -F 'ao evolve blocked --reason' "$SKILL"
  [ "$status" -eq 0 ]
}

@test "the write-stop-marker wire passes --mode loop (deterministic no-self-stop)" {
  run grep -F 'ao evolve write-stop-marker --marker dormant --reason "$REASON" --mode loop' "$SKILL"
  [ "$status" -eq 0 ]
}

@test "each wire keeps a fallback for ao without the subcommand (--help probe)" {
  # write-stop-marker + next-work both guard their call behind a --help probe so
  # an older ao falls back instead of erroring.
  run grep -F 'ao evolve write-stop-marker --help >/dev/null 2>&1' "$SKILL"
  [ "$status" -eq 0 ]
  run grep -F 'ao evolve next-work --help >/dev/null 2>&1' "$SKILL"
  [ "$status" -eq 0 ]
}
