#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/release-smoke-test.sh"
}

@test "release smoke is read-only and has no mutation escape hatch" {
  run rg -n 'ALLOW_AGENT_MUTATIONS|--no-cite|close-loop' "$SCRIPT"
  [ "$status" -eq 1 ]
}

@test "release smoke exercises current source-link and capability surfaces" {
  run rg -n 'skills link|capabilities --json|all generated leaf help' "$SCRIPT"
  [ "$status" -eq 0 ]
}

@test "release smoke verifies removed commands as tombstones" {
  run rg -n 'run_tombstone|goals trace tombstone|session memory tombstone|skills edit tombstone' "$SCRIPT"
  [ "$status" -eq 0 ]
}

@test "release smoke help is current" {
  run bash "$SCRIPT" --help
  [ "$status" -eq 0 ]
  [[ "$output" == *"retained read-only surface"* ]]
}
