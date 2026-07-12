#!/usr/bin/env bats
#
# tests for scripts/check-golden-path-drift.sh (age-a-plus-report-card-ieyp2.3)

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/check-golden-path-drift.sh"
  [ -r "$SCRIPT" ]
  FIX="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$FIX/scripts" "$FIX/docs/getting-started"
  cp "$SCRIPT" "$FIX/scripts/"
  chmod +x "$FIX/scripts/check-golden-path-drift.sh"
  # Minimal clean entry docs
  printf '# AgentOps\n\nFirst value: /plan → /implement → /validate\n' > "$FIX/README.md"
  printf '# Index\n\nSkill loop first.\n' > "$FIX/docs/index.md"
  printf '# Getting started\n\nUse /plan then /implement then /validate.\n' > "$FIX/docs/getting-started/index.md"
  printf '# First value\n\n/plan → /implement → /validate\n' > "$FIX/docs/first-value-path.md"
  RUN="$FIX/scripts/check-golden-path-drift.sh"
}

@test "clean entry docs PASS" {
  run bash "$RUN"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS"* ]]
}

@test "ao factory start as golden FAILS with file+line" {
  printf '# README\n\nRun ao factory start for your first session.\n' > "$FIX/README.md"
  run bash "$RUN"
  [ "$status" -eq 1 ]
  [[ "$output" == *"README.md"* ]]
  [[ "$output" == *"ao-factory-start-as-golden"* ]]
}

@test "ao verify as front door FAILS" {
  printf '# First value\n\nao verify is the front door of the product.\n' > "$FIX/docs/first-value-path.md"
  run bash "$RUN"
  [ "$status" -eq 1 ]
  [[ "$output" == *"ao-verify-as-front-door"* ]]
}

@test "past-tense mention of ao factory start is exempt" {
  printf '# README\n\nFormer path: ao factory start was removed; use the skill loop.\n' > "$FIX/README.md"
  run bash "$RUN"
  [ "$status" -eq 0 ]
}

@test "council-packet as first-value FAILS" {
  printf '# Getting started\n\nThe first-value journey is the council-packet path.\n' > "$FIX/docs/getting-started/index.md"
  run bash "$RUN"
  [ "$status" -eq 1 ]
  [[ "$output" == *"council-packet-as-first-value"* ]]
}
