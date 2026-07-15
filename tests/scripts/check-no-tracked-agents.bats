#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/check-no-tracked-agents.sh"
  FAKE_REPO="$BATS_TEST_TMPDIR/repo"
  mkdir -p "$FAKE_REPO"
  git -C "$FAKE_REPO" init -q
  git -C "$FAKE_REPO" config user.email test@example.com
  git -C "$FAKE_REPO" config user.name test
  printf '/.agents/\n' >"$FAKE_REPO/.gitignore"
  export NO_TRACKED_AGENTS_REPO_ROOT="$FAKE_REPO"
}

@test "ignored local agent state passes" {
  mkdir -p "$FAKE_REPO/.agents/local"
  printf '{}\n' >"$FAKE_REPO/.agents/local/state.json"
  run "$SCRIPT"
  [ "$status" -eq 0 ]
}

@test "any tracked repo-root agent state fails" {
  mkdir -p "$FAKE_REPO/.agents/rpi"
  printf '{}\n' >"$FAKE_REPO/.agents/rpi/next-work.jsonl"
  git -C "$FAKE_REPO" add -f .agents/rpi/next-work.jsonl
  run "$SCRIPT"
  [ "$status" -eq 1 ]
  [[ "$output" == *".agents/rpi/next-work.jsonl"* ]]
}

@test "a root re-include fails" {
  printf '/.agents/\n!/.agents/evolve/\n' >"$FAKE_REPO/.gitignore"
  run "$SCRIPT"
  [ "$status" -eq 1 ]
  [[ "$output" == *"must not re-include"* ]]
}

@test "nested test fixtures are not repo-root agent state" {
  mkdir -p "$FAKE_REPO/cli/cmd/testdata/example/.agents"
  printf '{}\n' >"$FAKE_REPO/cli/cmd/testdata/example/.agents/fixture.json"
  git -C "$FAKE_REPO" add cli/cmd/testdata/example/.agents/fixture.json
  run "$SCRIPT"
  [ "$status" -eq 0 ]
}
