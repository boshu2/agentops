#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
}

@test "normative CLI README rejects retired runtime paths" {
  run rg -n 'ao (factory start|rpi phased|codex stop|orchestrate)|/rpi ' "$REPO_ROOT/cli/README.md"
  [ "$status" -eq 1 ]
}

@test "explicitly historical prose remains outside the normative rejection" {
  fixture="$BATS_TEST_TMPDIR/historical.md"
  printf '%s\n' '<!-- agentops: historical -->' 'The retired `ao factory start` path existed before 3.0.' >"$fixture"
  run rg -n '^ao (factory start|rpi phased|codex stop)' "$fixture"
  [ "$status" -eq 1 ]
}
