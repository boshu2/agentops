#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
}

@test "Gas City executor static contract is green" {
  run "$REPO_ROOT/scripts/check-gc-executor.sh"
  [ "$status" -eq 0 ] || { printf '%s\n' "$output"; false; }
}
