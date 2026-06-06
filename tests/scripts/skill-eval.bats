#!/usr/bin/env bats
# Acceptance surface for ag-yzoz: scripts/skill-eval.sh gates a skill's
# SKILL.md through Jeff Emanuel's `ms` (meta_skill) lint + validate.
#
# Contract under test:
#   (1) The planted bad fixture (AWS key + empty description + missing required
#       metadata) exits NON-ZERO, naming all three findings.
#   (2) A fixed (good) fixture exits 0.
#   (3) With `ms` renamed off PATH the script HARD-FAILS (loud ::error::,
#       non-zero) — it never skips-and-passes.
#
# The bad/good fixtures are committed under skills/_fixtures/ (planted, not
# real skills). Tests requiring `ms` skip cleanly when ms is unavailable, but
# the ms-absent hard-fail test (3) runs unconditionally — it is the whole point.

setup() {
	REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
	SCRIPT="$REPO_ROOT/scripts/skill-eval.sh"
	BAD="$REPO_ROOT/skills/_fixtures/bad-skill/SKILL.md"
	GOOD="$REPO_ROOT/skills/_fixtures/good-skill/SKILL.md"
}

# Skip a test when the real `ms` binary is not installed.
require_ms() {
	command -v ms >/dev/null 2>&1 || skip "ms (meta_skill) not on PATH"
}

@test "script exists and is executable" {
	[ -f "$SCRIPT" ]
	[ -x "$SCRIPT" ]
}

@test "--help exits 0 and documents the blocking-vs-annotate contract" {
	run bash "$SCRIPT" --help
	[ "$status" -eq 0 ]
	[[ "$output" == *"Blocking rules"* ]]
	[[ "$output" == *"no-secrets"* ]]
	[[ "$output" == *"never skips-and-passes"* ]]
}

@test "missing argument exits 2 (usage error)" {
	run bash "$SCRIPT"
	[ "$status" -eq 2 ]
}

@test "unknown skill id exits 2" {
	require_ms
	run bash "$SCRIPT" no-such-skill-xyzzy
	[ "$status" -eq 2 ]
}

# (1) bad fixture -> non-zero, naming all three findings.
@test "bad fixture exits non-zero naming secret, empty description, and missing metadata" {
	require_ms
	[ -f "$BAD" ]
	run bash "$SCRIPT" "$BAD"
	[ "$status" -ne 0 ]
	# AWS key / secret finding
	[[ "$output" == *"no-secrets"* ]]
	[[ "$output" == *"AWS Access Key"* ]]
	# missing required metadata (id/name) finding
	[[ "$output" == *"required-metadata"* ]]
	[[ "$output" == *"'id' field"* || "$output" == *"'name' field"* ]]
	# empty description finding
	[[ "$output" == *"description"* ]]
	# the gate-failed banner is present
	[[ "$output" == *"BLOCKING findings"* ]]
}

# bad fixture is also resolvable by skill-id (nested under skills/).
@test "bad fixture resolves by nested skill id and still fails" {
	require_ms
	run bash "$SCRIPT" _fixtures/bad-skill
	[ "$status" -ne 0 ]
	[[ "$output" == *"BLOCKING findings"* ]]
}

# (2) good fixture -> exit 0.
@test "good fixture exits 0" {
	require_ms
	[ -f "$GOOD" ]
	run bash "$SCRIPT" "$GOOD"
	[ "$status" -eq 0 ]
	[[ "$output" == *"PASS"* ]]
}

@test "good fixture resolves by nested skill id and passes" {
	require_ms
	run bash "$SCRIPT" _fixtures/good-skill
	[ "$status" -eq 0 ]
}

# (3) ms off PATH -> loud hard-fail, NOT a pass. Runs unconditionally.
@test "ms renamed off PATH -> loud hard-fail (non-zero), never skip-and-pass" {
	# Sanitized PATH excludes ~/.cargo/bin (where ms lives), so `command -v ms`
	# fails inside the script — simulating ms being unavailable.
	run env PATH="/usr/bin:/bin" bash "$SCRIPT" "$GOOD"
	[ "$status" -ne 0 ]
	# Must be the loud tooling-unavailable code, never 0 (skip-and-pass).
	[ "$status" -eq 3 ]
	[[ "$output" == *"::error::"* ]]
	[[ "$output" == *"NOT on PATH"* ]]
	[[ "$output" == *"skip-and-pass"* ]]
}

# Explicit MS_BIN override to a non-existent binary -> same hard-fail.
@test "MS_BIN pointing at a missing binary -> hard-fail, not skip-and-pass" {
	run env MS_BIN="ms-definitely-not-installed-xyzzy" bash "$SCRIPT" "$GOOD"
	[ "$status" -eq 3 ]
	[[ "$output" == *"::error::"* ]]
	[[ "$output" == *"NOT on PATH"* ]]
}
