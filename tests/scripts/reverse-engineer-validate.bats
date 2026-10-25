#!/usr/bin/env bats
# Runs the reverse-engineer skill's hermetic behavioral checks in CI. They need
# python3 and git only (rg when present, grep otherwise); binary mode, which
# needs Go, stays in skills/reverse-engineer/scripts/self_test.sh.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
}

@test "reverse-engineer validate.sh passes (no Go or network)" {
  run bash "$REPO_ROOT/skills/reverse-engineer/scripts/validate.sh"
  [ "$status" -eq 0 ]
}
