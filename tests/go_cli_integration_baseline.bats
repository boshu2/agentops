#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  RESCUE_REF="refs/heads/rescue/age-nw28h.7.8-pre-integration-20260712"
  PRE_INTEGRATION_SHA="e68866553583125c0a08727dad40417e14450b75"
}

: <<'EXACT_ADMISSION_COLLECTOR'
@test "integration baseline rejects stale ancestry"
EXACT_ADMISSION_COLLECTOR

@test "integration baseline rejects stale ancestry" {
  command -v git >/dev/null
  test -x "$REPO_ROOT/scripts/check-go-cli-integration-baseline.sh"

  run bash -n "$REPO_ROOT/scripts/check-go-cli-integration-baseline.sh"
  [ "$status" -eq 0 ]

  run git -C "$REPO_ROOT" rev-parse --verify "$RESCUE_REF"
  [ "$status" -eq 0 ]
  [ "$output" = "$PRE_INTEGRATION_SHA" ]

  run git -C "$REPO_ROOT" merge-base --is-ancestor "$RESCUE_REF" HEAD
  [ "$status" -eq 0 ]

  run git -C "$REPO_ROOT" rev-parse --verify origin/main
  [ "$status" -eq 0 ]

  run git -C "$REPO_ROOT" merge-base --is-ancestor origin/main HEAD
  [ "$status" -eq 0 ]
}
