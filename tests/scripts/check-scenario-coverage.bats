#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT="$REPO_ROOT/scripts/check-scenario-coverage.sh"
  TMP_DIR="$(mktemp -d)"
  FAKE_REPO="$TMP_DIR/repo"
  mkdir -p "$FAKE_REPO/scripts" "$FAKE_REPO/tests" "$FAKE_REPO/work"
  cp "$SCRIPT" "$FAKE_REPO/scripts/check-scenario-coverage.sh"
  chmod +x "$FAKE_REPO/scripts/check-scenario-coverage.sh"
  FAKE_SCRIPT="$FAKE_REPO/scripts/check-scenario-coverage.sh"
  printf '#!/usr/bin/env bash\nTestCoversScenario() { :; }\nexit 0\n' > "$FAKE_REPO/tests/pass.sh"
  printf '#!/usr/bin/env bash\nexit 1\n' > "$FAKE_REPO/tests/fail.sh"
}

teardown() { rm -rf "$TMP_DIR"; }

write_feature() {
  local path="$1" first_tag="$2" second_tag="${3-$2}"
  printf 'Feature: example\n\n  %s\n  Scenario: first\n    Given x\n    Then y\n\n  %s\n  Scenario: second\n    Given x\n    Then y\n' \
    "$first_tag" "$second_tag" > "$path"
}

@test "script is executable" { [ -x "$SCRIPT" ]; }

@test "all linked scenarios pass" {
  write_feature "$FAKE_REPO/work/a.feature" '@covered-by:tests/pass.sh'
  run bash "$FAKE_SCRIPT" "$FAKE_REPO/work/a.feature"
  [ "$status" -eq 0 ]
  [[ "$output" == *"2/2"* ]]
}

@test "an unlinked scenario fails" {
  write_feature "$FAKE_REPO/work/a.feature" '@covered-by:tests/pass.sh' ''
  run bash "$FAKE_SCRIPT" "$FAKE_REPO/work/a.feature"
  [ "$status" -eq 1 ]
  [[ "$output" == *"no covering test"* ]]
}

@test "a missing linked file fails" {
  write_feature "$FAKE_REPO/work/a.feature" '@covered-by:tests/missing.sh'
  run bash "$FAKE_SCRIPT" "$FAKE_REPO/work/a.feature"
  [ "$status" -eq 1 ]
  [[ "$output" == *"test path does not exist"* ]]
}

@test "a named test must exist" {
  write_feature "$FAKE_REPO/work/a.feature" '@covered-by:tests/pass.sh::MissingName'
  run bash "$FAKE_SCRIPT" "$FAKE_REPO/work/a.feature"
  [ "$status" -eq 1 ]
  [[ "$output" == *"not found"* ]]
}

@test "a named test resolves" {
  write_feature "$FAKE_REPO/work/a.feature" '@covered-by:tests/pass.sh::TestCoversScenario'
  run bash "$FAKE_SCRIPT" "$FAKE_REPO/work/a.feature"
  [ "$status" -eq 0 ]
}

@test "run requires the linked test to pass" {
  write_feature "$FAKE_REPO/work/a.feature" '@covered-by:tests/fail.sh'
  run bash "$FAKE_SCRIPT" --run "$FAKE_REPO/work/a.feature"
  [ "$status" -eq 1 ]
  [[ "$output" == *"did not pass"* ]]
}

@test "markdown input counts only its Scenarios section" {
  printf '## Scenarios\n@covered-by:tests/pass.sh\nScenario: real\n Given x\n Then y\n\n## Notes\nScenario: prose\n' > "$FAKE_REPO/work/a.md"
  run bash "$FAKE_SCRIPT" --json "$FAKE_REPO/work/a.md"
  [ "$status" -eq 0 ]
  [[ "$output" == *'"scenarios_total":1'* ]]
}

@test "fenced scenarios are ignored" {
  printf 'Feature: example\n```\n@covered-by:tests/pass.sh\nScenario: fenced\n```\n' > "$FAKE_REPO/work/a.feature"
  run bash "$FAKE_SCRIPT" "$FAKE_REPO/work/a.feature"
  [ "$status" -eq 1 ]
  [[ "$output" == *"no scenarios found"* ]]
}

@test "stdin and JSON are supported" {
  write_feature "$FAKE_REPO/work/a.feature" '@covered-by:tests/pass.sh'
  run bash -c "cat '$FAKE_REPO/work/a.feature' | bash '$FAKE_SCRIPT' --json -"
  [ "$status" -eq 0 ]
  [[ "$output" == *'"result":"pass"'* ]]
}

@test "missing source is misuse" {
  run bash "$FAKE_SCRIPT"
  [ "$status" -eq 2 ]
}

@test "removed lifecycle flags are rejected" {
  run bash "$FAKE_SCRIPT" --bead old-state
  [ "$status" -eq 2 ]
}
