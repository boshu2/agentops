#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/prune-agents.sh"
  FIXTURE_ROOT="$BATS_TEST_TMPDIR/repo"
  EXPECTED_LEGACY="$BATS_TEST_TMPDIR/expected-legacy"
  export AGENTOPS_REPO_ROOT="$FIXTURE_ROOT"
  mkdir -p "$FIXTURE_ROOT/.agents/handoff" "$FIXTURE_ROOT/.agents/mto-handoff" \
    "$FIXTURE_ROOT/.agents/ao/handoff" "$EXPECTED_LEGACY"
}

populate_canonical_handoffs() {
  local i minute canonical
  for i in $(seq 1 12); do
    canonical="$FIXTURE_ROOT/.agents/ao/handoff/handoff-20260816T0100${i}.000000000Z.json"
    printf 'canonical-%s\n' "$i" > "$canonical"
    # Make retention order deterministic without relying on filename order.
    minute="$(printf '%02d' "$((i - 1))")"
    touch -t "2026081601${minute}.00" "$canonical"
  done
}

@test "execute mode prunes canonical handoffs but preserves every legacy artifact byte-for-byte" {
  local i minute legacy
  printf '{"protocol":"mto-recurrence"}\n' > "$FIXTURE_ROOT/.agents/mto-handoff/recurrence.json"
  for i in $(seq 1 12); do
    legacy="$FIXTURE_ROOT/.agents/handoff/handoff-20260816T0000${i}.000000000Z.json"
    printf '{"schema_version":1,"payload":"legacy-%s"}\n\n' "$i" > "$legacy"
    cp "$legacy" "$EXPECTED_LEGACY/$(basename "$legacy")"
    # Make retention order deterministic without relying on filename order.
    minute="$(printf '%02d' "$((i - 1))")"
    touch -t "2026081601${minute}.00" "$legacy"
  done
  populate_canonical_handoffs

  run "$SCRIPT" --execute
  [ "$status" -eq 0 ]

  [ "$(find "$FIXTURE_ROOT/.agents/ao/handoff" -maxdepth 1 -type f | wc -l | tr -d ' ')" -eq 10 ]
  [ "$(find "$FIXTURE_ROOT/.agents/handoff" -maxdepth 1 -type f | wc -l | tr -d ' ')" -eq 12 ]
  [ "$(cat "$FIXTURE_ROOT/.agents/mto-handoff/recurrence.json")" = '{"protocol":"mto-recurrence"}' ]
  [[ "$output" == *"Files deleted: 2"* ]]
  [[ "$output" == *"handoff/ mto-handoff/"* ]]
  for i in $(seq 1 12); do
    legacy="$FIXTURE_ROOT/.agents/handoff/handoff-20260816T0000${i}.000000000Z.json"
    [ -f "$legacy" ]
    cmp -s "$EXPECTED_LEGACY/$(basename "$legacy")" "$legacy"
  done
}

@test "dry run reports exactly two canonical handoffs and deletes nothing" {
  populate_canonical_handoffs

  run "$SCRIPT"
  [ "$status" -eq 0 ]

  [ "$(find "$FIXTURE_ROOT/.agents/ao/handoff" -maxdepth 1 -type f | wc -l | tr -d ' ')" -eq 12 ]
  [[ "$output" == *"Files that would be deleted: 2"* ]]
}

@test "execute refuses an intermediate canonical symlink and leaves outside bytes unchanged" {
  local outside expected
  outside="$BATS_TEST_TMPDIR/outside"
  expected="$BATS_TEST_TMPDIR/outside.expected"
  mkdir -p "$outside/ao/handoff"
  printf 'outside sentinel\n' > "$outside/ao/handoff/sentinel"
  cp "$outside/ao/handoff/sentinel" "$expected"

  rmdir "$FIXTURE_ROOT/.agents/ao/handoff" "$FIXTURE_ROOT/.agents/ao"
  ln -s "$outside/ao" "$FIXTURE_ROOT/.agents/ao"

  run "$SCRIPT" --execute
  [ "$status" -ne 0 ]
  [[ "$output" == *"canonical handoff path component is a symlink"* ]]
  cmp -s "$expected" "$outside/ao/handoff/sentinel"
  [ "$(find "$outside" -type f | wc -l | tr -d ' ')" -eq 1 ]
}
