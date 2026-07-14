#!/usr/bin/env bats
# Acceptance surface for deterministic, digest-safe goal-design packet helpers.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  TOOL="$REPO_ROOT/scripts/goal-design-packet.py"
  CHECKER="$REPO_ROOT/scripts/check-goal-design-packet.sh"
  if ! command -v python3 >/dev/null 2>&1 || ! python3 -c 'import yaml, jsonschema' >/dev/null 2>&1; then
    HAVE_SCHEMA_DEPS=0
  else
    HAVE_SCHEMA_DEPS=1
  fi
}

new_packet() {
  local slug="$1"
  "$TOOL" new "$slug" \
    --output-root "$BATS_TEST_TMPDIR/.agents/goal-design" \
    --objective "Shape deterministic intent for $slug" \
    --scenario-name "Shape deterministic intent for $slug" \
    --first-failing-proof "bats tests/scripts/goal-design-packet.bats" \
    --write-scope "scripts/goal-design-packet.py"
}

@test "new creates a checker-clean packet with matching digest" {
  if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
  run new_packet generated-packet
  [ "$status" -eq 0 ]
  [[ "$output" == *"goal-design packet valid"* ]]

  intent="$BATS_TEST_TMPDIR/.agents/goal-design/generated-packet/intent.md"
  driver="$BATS_TEST_TMPDIR/.agents/goal-design/generated-packet/driver.md"
  expected="$(sha256sum "$intent" | awk '{print $1}')"
  grep -Fq "sha256: $expected" "$driver"
  grep -Fq "checker_command:" "$driver"
  ! grep -Eq 'independent_(gate|validator)|required_verdict|validated' "$intent" "$driver"
}

@test "refresh-digest repairs stale intent identity" {
  if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
  new_packet stale-packet >/dev/null
  packet="$BATS_TEST_TMPDIR/.agents/goal-design/stale-packet"
  printf '\nDigest-changing edit.\n' >>"$packet/intent.md"

  run "$CHECKER" "$packet"
  [ "$status" -ne 0 ]
  [[ "$output" == *"driver intent_ref.sha256 is stale"* ]]

  run "$TOOL" refresh-digest "$packet"
  [ "$status" -eq 0 ]
  [[ "$output" == *"goal-design packet valid"* ]]
}

@test "check delegates to the canonical packet checker" {
  if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
  run "$TOOL" check "$REPO_ROOT/tests/fixtures/goal-design/mismatched-slug"
  [ "$status" -ne 0 ]
  [[ "$output" == *"slug mismatch"* ]]
}

@test "prompt accepts a checker-clean draft without semantic readiness state" {
  if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
  new_packet dispatch-packet >/dev/null
  packet="$BATS_TEST_TMPDIR/.agents/goal-design/dispatch-packet"

  run "$TOOL" prompt "$packet"
  [ "$status" -eq 0 ]
  [[ "$output" == *"$packet/intent.md"* ]]
  [[ "$output" == *"checker-clean intent"* ]]
  [[ "$output" == *"Premortem judges the exact final plan"* ]]
  [[ "$output" != *"DRAFT PACKET"* ]]
  [ "${#output}" -lt 4000 ]
}

@test "prompt keeps the hard character ceiling" {
  if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
  new_packet ceiling-packet >/dev/null
  packet="$BATS_TEST_TMPDIR/.agents/goal-design/ceiling-packet"

  run "$TOOL" prompt "$packet" --max-chars 50
  [ "$status" -ne 0 ]
  [[ "$output" == *"max-chars"* ]]

  run "$TOOL" prompt "$packet" --max-chars 5000
  [ "$status" -ne 0 ]
  [[ "$output" == *"1..4000"* ]]
}

@test "prompt refuses a checker-dirty packet" {
  if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
  new_packet stale-dispatch >/dev/null
  packet="$BATS_TEST_TMPDIR/.agents/goal-design/stale-dispatch"
  printf '\nStale-making edit.\n' >>"$packet/intent.md"

  run "$TOOL" prompt "$packet"
  [ "$status" -ne 0 ]
  [[ "$output" == *"checker failed"* ]]
}

@test "semantic stamping command and nested governor text are absent" {
  run "$TOOL" --help
  [ "$status" -eq 0 ]
  [[ "$output" != *"mark-validated"* ]]

  run rg -n 'AUTO-REDO|HELPER-UNSTUCK|one bounded helper|ao plan-pawl|Last validation verdict' \
    "$REPO_ROOT/scripts/goal-design-packet.py" "$REPO_ROOT/skills/goal-design/SKILL.md"
  [ "$status" -eq 1 ]
}

@test "close records evidence-bound candidate disposition from checker-clean draft" {
  if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
  new_packet close-happy >/dev/null
  packet="$BATS_TEST_TMPDIR/.agents/goal-design/close-happy"

  run "$TOOL" close "$packet" --candidate "B1=closed:commit abc123"
  [ "$status" -eq 0 ]
  grep -q '^status: closed' "$packet/intent.md"
  grep -q '^status: closed' "$packet/driver.md"
  grep -Fq -- '- Disposition B1: closed - commit abc123' "$packet/driver.md"
}

@test "close rejects missing candidate dispositions" {
  if [ "$HAVE_SCHEMA_DEPS" -eq 0 ]; then skip "python3 yaml/jsonschema unavailable"; fi
  new_packet close-missing >/dev/null
  packet="$BATS_TEST_TMPDIR/.agents/goal-design/close-missing"

  run "$TOOL" close "$packet"
  [ "$status" -ne 0 ]
  [[ "$output" == *"missing: B1"* ]]
}
