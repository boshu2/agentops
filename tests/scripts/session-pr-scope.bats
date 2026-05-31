#!/usr/bin/env bats
# ag-o5xp: session-pr-scope.sh is the hookless replacement for the removed
# hooks/session-pr-counter.sh (deleted in the #511 hookless teardown). It counts
# the current user's recent PRs and emits a session-scope verdict. These cases
# exercise the verdict thresholds, --count/--json modes, block-mode exit code,
# and fail-open behavior deterministically via an injectable fake `gh` on PATH
# (so they never touch the network or depend on real PR history).

setup() {
  SCRIPT="$BATS_TEST_DIRNAME/../../scripts/session-pr-scope.sh"
  STUB="$(mktemp -d)"
  # Fake gh: emits a JSON array of SET_PR_COUNT objects, so the real `jq -r length`
  # in the script yields a deterministic count. Ignores all args (the script only
  # cares about the JSON array length).
  cat >"$STUB/gh" <<'EOF'
#!/usr/bin/env bash
n="${SET_PR_COUNT:-0}"
printf '['
i=0
while [ "$i" -lt "$n" ]; do
  [ "$i" -gt 0 ] && printf ','
  printf '{"number":%d}' "$i"
  i=$((i + 1))
done
printf ']\n'
EOF
  chmod +x "$STUB/gh"
  PATH="$STUB:$PATH"
}

teardown() { rm -rf "$STUB"; }

@test "OK below warn threshold (2 PRs, threshold 5)" {
  SET_PR_COUNT=2 run bash "$SCRIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"OK"* ]]
  [[ "$output" == *"2 PR"* ]]
}

@test "WARN at threshold-1 (4 PRs, threshold 5)" {
  SET_PR_COUNT=4 run bash "$SCRIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"WARN"* ]]
  [[ "$output" == *"post-mortem"* ]]
}

@test "WARN (not block) at/over threshold without block mode (6 PRs)" {
  SET_PR_COUNT=6 run bash "$SCRIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"WARN"* ]]
}

@test "BLOCK + exit 2 over threshold with block mode opted in (5 PRs)" {
  SET_PR_COUNT=5 AGENTOPS_SESSION_PR_BLOCK=1 run bash "$SCRIPT"
  [ "$status" -eq 2 ]
  [[ "$output" == *"BLOCK"* ]]
}

@test "--count emits the raw integer" {
  SET_PR_COUNT=3 run bash "$SCRIPT" --count
  [ "$status" -eq 0 ]
  [ "$output" = "3" ]
}

@test "--json reports verdict and over flag" {
  SET_PR_COUNT=5 run bash "$SCRIPT" --json
  [ "$status" -eq 0 ]
  [[ "$output" == *'"verdict":"warn"'* ]]
  [[ "$output" == *'"over":true'* ]]
  [[ "$output" == *'"count":5'* ]]
}

@test "custom SESSION_PR_THRESHOLD shifts the warn edge" {
  # threshold 3 => warn at count>=2
  SET_PR_COUNT=2 SESSION_PR_THRESHOLD=3 run bash "$SCRIPT"
  [ "$status" -eq 0 ]
  [[ "$output" == *"WARN"* ]]
}

@test "fail-open: gh failure yields verdict=unknown, exit 0" {
  # Fake gh that errors → empty/unparseable count → fail open.
  cat >"$STUB/gh" <<'EOF'
#!/usr/bin/env bash
exit 1
EOF
  chmod +x "$STUB/gh"
  run bash "$SCRIPT" --json
  [ "$status" -eq 0 ]
  [[ "$output" == *'"verdict":"unknown"'* ]]
}
