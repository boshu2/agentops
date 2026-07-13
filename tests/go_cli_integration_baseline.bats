#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
}

@test "integration baseline rejects stale ancestry" {
  command -v git >/dev/null
  test -x "$REPO_ROOT/scripts/check-go-cli-integration-baseline.sh"

  run bash -n "$REPO_ROOT/scripts/check-go-cli-integration-baseline.sh"
  [ "$status" -eq 0 ]

  run rg -n --fixed-strings \
    'scripts/check-go-cli-compatibility.sh --oracle-version current --verify-frozen --profiles default,flywheel,legacy,combined --family ' \
    "$REPO_ROOT/scripts/check-go-cli-integration-baseline.sh"
  [ "$status" -eq 0 ]

  run bash "$REPO_ROOT/scripts/check-go-cli-integration-baseline.sh"
  printf '%s\n' "$output"
  [ "$status" -eq 0 ]
}
