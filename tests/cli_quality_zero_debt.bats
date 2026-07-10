#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
}

@test "CLI Go lint has zero findings" {
  run bash -c "cd '$REPO_ROOT/cli' && ../scripts/golangci-lint-v2.sh run ./..."
  [ "$status" -eq 0 ]
  [ "$output" = "0 issues." ]
}

@test "CLI generated reference has zero drift" {
  run bash "$REPO_ROOT/scripts/generate-cli-reference.sh" --check
  [ "$status" -eq 0 ]
}
