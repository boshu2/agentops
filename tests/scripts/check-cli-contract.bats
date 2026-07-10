#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
}

@test "CLI contract aggregate gate is executable and green" {
  run bash "$REPO_ROOT/scripts/check-cli-contract.sh"
  [ "$status" -eq 0 ]
}
