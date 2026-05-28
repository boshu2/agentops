#!/usr/bin/env bats
# Regression tests for scripts/lint-evidence-lines.sh (soc-l340).
#
# The linter must (1) extract Evidence: lines with the same regex CI uses,
# (2) classify each claim against the AP#7 failure modes we've encountered
# in practice, (3) exit non-zero only when blocking issues are present
# (advisory issues need --strict to fail), and (4) accept --stdin / --body /
# <pr-number> input modes uniformly.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  SCRIPT="$REPO_ROOT/scripts/lint-evidence-lines.sh"
  TMP="$(mktemp -d)"
  ORIG_DIR="$PWD"
}

teardown() {
  cd "$ORIG_DIR" 2>/dev/null || true
  rm -rf "$TMP"
}

# Pipe a body through stdin mode.
run_stdin() {
  run bash -c "printf '%s' '$1' | $SCRIPT --stdin ${2:-}"
}

@test "exits 0 when no Evidence: lines present" {
  run_stdin "no trailers in this body"
  [ "$status" -eq 0 ]
  [[ "$output" == *"no Evidence: lines"* ]]
}

@test "exits 0 on clean bare-path Evidence: lines" {
  run_stdin "Evidence: scripts/foo.sh
Evidence: tests/scripts/foo.bats"
  [ "$status" -eq 0 ]
  [[ "$output" == *"0 blocking"* ]]
  [[ "$output" == *"OK"* ]]
}

@test "blocks on parenthetical addendum (the canonical AP#7 trap)" {
  run_stdin "Evidence: scripts/foo.sh (10/10 passing)"
  [ "$status" -eq 1 ]
  [[ "$output" == *"FAIL"* ]]
  [[ "$output" == *"parenthetical"* ]]
  [[ "$output" == *"(10/10 passing)"* ]]
}

@test "blocks on markdown bold inside the path" {
  run_stdin "Evidence: **scripts/foo.sh**"
  [ "$status" -eq 1 ]
  [[ "$output" == *"markdown"* ]]
}

@test "blocks on backtick code-fence inside the claim" {
  run_stdin 'Evidence: `scripts/foo.sh`'
  [ "$status" -eq 1 ]
  [[ "$output" == *"markdown"* ]]
}

@test "blocks on pipe character (markdown table sin)" {
  run_stdin "Evidence: scripts/foo.sh | tests/foo.bats"
  [ "$status" -eq 1 ]
  [[ "$output" == *"pipe"* ]]
}

@test "blocks on empty Evidence: line" {
  run_stdin "Evidence:"
  [ "$status" -eq 1 ]
  [[ "$output" == *"empty"* ]]
}

@test "blocks on prose-keyword openers (see/cf/note)" {
  run_stdin "Evidence: see scripts/foo.sh for details"
  [ "$status" -eq 1 ]
  [[ "$output" == *"prose"* ]]
}

@test "trailing whitespace is advisory (passes by default, blocks under --strict)" {
  printf 'Evidence: scripts/foo.sh \n' > "$TMP/body.md"
  run "$SCRIPT" --body "$TMP/body.md"
  [ "$status" -eq 0 ]
  [[ "$output" == *"WARN"* ]] || [[ "$output" == *"trailing-ws"* ]]
  # Strict mode promotes advisory to blocking.
  run "$SCRIPT" --body "$TMP/body.md" --strict
  [ "$status" -eq 1 ]
}

@test "--json output is valid and reports counts" {
  printf 'Evidence: scripts/foo.sh\nEvidence: bad (paren)\n' > "$TMP/body.md"
  run "$SCRIPT" --body "$TMP/body.md" --json
  [ "$status" -eq 1 ]
  echo "$output" | jq -e '.blocking == 1 and (.claims | length) == 2' >/dev/null
}

@test "--body reads from a file path" {
  printf 'Evidence: scripts/x.sh\n' > "$TMP/body.md"
  run "$SCRIPT" --body "$TMP/body.md"
  [ "$status" -eq 0 ]
  [[ "$output" == *"scripts/x.sh"* ]]
}

@test "missing body file exits 3 with clear error" {
  run "$SCRIPT" --body "$TMP/does-not-exist.md"
  [ "$status" -eq 3 ]
  [[ "$output" == *"cannot read body file"* ]]
}

@test "no <pr-number> in pr-mode exits 2 (usage)" {
  run "$SCRIPT"
  [ "$status" -eq 2 ]
  [[ "$output" == *"missing"* ]] || [[ "$output" == *"pr-number"* ]] || [[ "$output" == *"Usage"* ]]
}

@test "extraction regex matches CI exactly (leading-space tolerance)" {
  # CI uses `sed -n 's/^Evidence:[[:space:]]*//p'`. Lines starting with
  # whitespace BEFORE "Evidence:" must NOT match — they're not trailer lines.
  run_stdin "  Evidence: should-not-be-matched
Evidence: scripts/real.sh"
  [ "$status" -eq 0 ]
  # Only one claim should have been classified.
  [[ "$output" == *"1 Evidence:"* ]]
  [[ "$output" == *"scripts/real.sh"* ]]
  [[ "$output" != *"should-not-be-matched"* ]]
}

@test "multiple blocking issues per body are all reported" {
  run_stdin "Evidence: a (paren)
Evidence: b | pipe
Evidence: c"
  [ "$status" -eq 1 ]
  [[ "$output" == *"3 Evidence:"* ]]
  [[ "$output" == *"2 blocking"* ]]
}
