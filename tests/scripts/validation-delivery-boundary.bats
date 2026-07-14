#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  CHECKER="$REPO_ROOT/scripts/check-validation-delivery-boundary.sh"
  FIXTURE_ROOT="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$FIXTURE_ROOT/skills/crank" "$FIXTURE_ROOT/skills/rpi/references"
}

run_reference_check() {
  run bash "$CHECKER" --check-crank-references \
    "$FIXTURE_ROOT" "$FIXTURE_ROOT/skills/crank/SKILL.md"
}

@test "accepts a valid cross-skill reference resolved from the source document" {
  printf '%s\n' '# Persistent Pull-Flow Governor' \
    >"$FIXTURE_ROOT/skills/rpi/references/pull-flow-governor.md"
  printf '%s\n' \
    '[run governor](../rpi/references/pull-flow-governor.md)' \
    >"$FIXTURE_ROOT/skills/crank/SKILL.md"

  run_reference_check

  [ "$status" -eq 0 ]
}

@test "rejects a truly missing relative reference" {
  printf '%s\n' '[missing](references/not-there.md)' \
    >"$FIXTURE_ROOT/skills/crank/SKILL.md"

  run_reference_check

  [ "$status" -eq 1 ]
  [[ "$output" == *"Crank links missing reference: references/not-there.md"* ]]
}

@test "rejects a reachable reference that owns delivery" {
  mkdir -p "$FIXTURE_ROOT/skills/crank/references"
  printf '%s\n' '[delivery](references/delivery.md)' \
    >"$FIXTURE_ROOT/skills/crank/SKILL.md"
  printf '%s\n' 'Run `bd close` after validation.' \
    >"$FIXTURE_ROOT/skills/crank/references/delivery.md"

  run_reference_check

  [ "$status" -eq 1 ]
  [[ "$output" == *"reachable Crank reference retains delivery/tracker authority: skills/crank/references/delivery.md"* ]]
}
