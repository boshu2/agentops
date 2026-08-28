#!/usr/bin/env bats
#
# Behavioral spec for scripts/check-gate-tightening-ratchet.sh — the
# gate.tightening-ratchet advisory gate.
#
# The acceptance this pins: a diff that LOOSENS a gate threshold, flips a
# registered check from blocking to advisory, drops shell strictness, adds a
# suppression, or deletes a gate file must FAIL over BASE_REF..HEAD unless a
# `Gate-Loosen-Reason:` trailer appears in a commit body in range. TIGHTENING
# the same knob must always pass — a detector that merely notices "a number
# changed" fails the tighten cases here.
#
# Each test builds a throwaway git repo carrying the two governed path classes
# (cli/internal/gates/** and top-level scripts/check-*.sh), commits a baseline,
# captures its SHA, mutates + commits, then runs the gate with
# GATE_TIGHTENING_ROOT pointed at the throwaway tree (the script sources
# scripts/lib/preamble.sh, whose REPO_ROOT is anchored at THIS repo — the seam
# is what lets the spec drive a synthetic diff).

setup() {
	REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
	SCRIPT="$REPO_ROOT/scripts/check-gate-tightening-ratchet.sh"

	# Git injects GIT_DIR/GIT_WORK_TREE into hook-launched processes; a fixture
	# `git init` under a leaked GIT_DIR rewrites the SHARED config (ek8v).
	unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX GIT_OBJECT_DIRECTORY GIT_COMMON_DIR GIT_NAMESPACE

	TMP_DIR="$(mktemp -d)"
	WORK_REPO="$TMP_DIR/repo"
	git init -b main "$WORK_REPO" >/dev/null
	git -C "$WORK_REPO" config user.name "Test User"
	git -C "$WORK_REPO" config user.email "test@example.com"
	mkdir -p "$WORK_REPO/scripts" "$WORK_REPO/cli/internal/gates/checks"
}

teardown() {
	rm -rf "$TMP_DIR"
}

# write_gate_script <max-errors>
write_gate_script() {
	cat >"$WORK_REPO/scripts/check-thing.sh" <<EOF
#!/usr/bin/env bash
set -euo pipefail
MAX_ERRORS=$1
MIN_COVERAGE=80
echo "checking with budget \$MAX_ERRORS"
EOF
}

# write_seed <blocking>
write_seed() {
	cat >"$WORK_REPO/cli/internal/gates/checks/seed.go" <<EOF
package checks

func init() {
	add(Check{ID: "thing.check", Blocking: $1, Backing: "check-thing.sh"})
	add(Check{ID: "other.check", Blocking: true, Backing: "check-other.sh"})
}
EOF
}

commit_all() {
	git -C "$WORK_REPO" add -A
	git -C "$WORK_REPO" commit -q -m "$1"
}

baseline() {
	write_gate_script 5
	write_seed true
	commit_all "baseline"
	BASE="$(git -C "$WORK_REPO" rev-parse HEAD)"
}

run_gate() {
	run env GATE_TIGHTENING_ROOT="$WORK_REPO" BASE_REF="$BASE" bash "$SCRIPT"
}

@test "raising a max/budget threshold is LOOSENING and fails without a trailer" {
	baseline
	write_gate_script 20
	commit_all "raise the error budget"

	run_gate
	[ "$status" -eq 1 ]
	[[ "$output" == *"scripts/check-thing.sh"* ]]
	[[ "$output" == *"5"* ]]
	[[ "$output" == *"20"* ]]
	[[ "$output" == *"Gate-Loosen-Reason"* ]]
}

@test "lowering the same max/budget threshold is TIGHTENING and passes" {
	baseline
	write_gate_script 2
	commit_all "tighten the error budget"

	run_gate
	[ "$status" -eq 0 ]
	[[ "$output" == *"PASS"* ]]
}

@test "loosening WITH a Gate-Loosen-Reason trailer passes" {
	baseline
	write_gate_script 20
	git -C "$WORK_REPO" add -A
	git -C "$WORK_REPO" commit -q -m "raise the error budget

Gate-Loosen-Reason: upstream tool emits 15 unavoidable warnings; ratchet resumes next cycle"

	run_gate
	[ "$status" -eq 0 ]
	[[ "$output" == *"Gate-Loosen-Reason"* ]]
}

@test "lowering a min/required floor is LOOSENING and fails" {
	baseline
	cat >"$WORK_REPO/scripts/check-thing.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
MAX_ERRORS=5
MIN_COVERAGE=40
echo "checking with budget $MAX_ERRORS"
EOF
	commit_all "lower the coverage floor"

	run_gate
	[ "$status" -eq 1 ]
	[[ "$output" == *"MIN_COVERAGE"* ]]
}

@test "raising a min/required floor is TIGHTENING and passes" {
	baseline
	cat >"$WORK_REPO/scripts/check-thing.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
MAX_ERRORS=5
MIN_COVERAGE=95
echo "checking with budget $MAX_ERRORS"
EOF
	commit_all "raise the coverage floor"

	run_gate
	[ "$status" -eq 0 ]
	[[ "$output" == *"PASS"* ]]
}

@test "flipping a registered check from Blocking:true to Blocking:false is LOOSENING" {
	baseline
	write_seed false
	commit_all "demote thing.check to advisory"

	run_gate
	[ "$status" -eq 1 ]
	[[ "$output" == *"thing.check"* ]]
	[[ "$output" == *"blocking"* ]]
}

@test "dropping shell strictness from a gate script is LOOSENING" {
	baseline
	cat >"$WORK_REPO/scripts/check-thing.sh" <<'EOF'
#!/usr/bin/env bash
MAX_ERRORS=5
MIN_COVERAGE=80
echo "checking with budget $MAX_ERRORS"
EOF
	commit_all "drop set -euo pipefail"

	run_gate
	[ "$status" -eq 1 ]
	[[ "$output" == *"strictness"* ]]
}

@test "adding a fail-open '|| true' to a gate script is LOOSENING" {
	baseline
	cat >"$WORK_REPO/scripts/check-thing.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
MAX_ERRORS=5
MIN_COVERAGE=80
grep -q thing "$0" || true
echo "checking with budget $MAX_ERRORS"
EOF
	commit_all "swallow the grep failure"

	run_gate
	[ "$status" -eq 1 ]
	[[ "$output" == *"fail-open"* ]]
}

@test "adding a lint suppression to a gate file is LOOSENING" {
	baseline
	cat >"$WORK_REPO/cli/internal/gates/checks/seed.go" <<'EOF'
package checks

func init() {
	//nolint:gosec // trust me
	add(Check{ID: "thing.check", Blocking: true, Backing: "check-thing.sh"})
	add(Check{ID: "other.check", Blocking: true, Backing: "check-other.sh"})
}
EOF
	commit_all "suppress the linter"

	run_gate
	[ "$status" -eq 1 ]
	[[ "$output" == *"suppression"* ]]
}

@test "deleting a governed gate script is LOOSENING" {
	baseline
	rm "$WORK_REPO/scripts/check-thing.sh"
	commit_all "delete the gate script"

	run_gate
	[ "$status" -eq 1 ]
	[[ "$output" == *"DELETED"* ]]
	[[ "$output" == *"scripts/check-thing.sh"* ]]
}

@test "a pure addition to the registry (new advisory gate) passes" {
	baseline
	cat >"$WORK_REPO/cli/internal/gates/checks/seed.go" <<'EOF'
package checks

func init() {
	add(Check{ID: "thing.check", Blocking: true, Backing: "check-thing.sh"})
	add(Check{ID: "other.check", Blocking: true, Backing: "check-other.sh"})
	add(Check{ID: "new.check", Blocking: false, Backing: "check-new.sh"})
}
EOF
	commit_all "register a new advisory gate"

	run_gate
	[ "$status" -eq 0 ]
	[[ "$output" == *"PASS"* ]]
}

@test "an ungoverned file (docs) is never scanned" {
	baseline
	mkdir -p "$WORK_REPO/docs"
	printf 'MAX_ERRORS=5000\n' >"$WORK_REPO/docs/notes.md"
	commit_all "doc-only change"

	run_gate
	[ "$status" -eq 0 ]
	[[ "$output" == *"no governed"* ]]
}

@test "unresolvable BASE_REF: fail-open SKIP" {
	baseline
	write_gate_script 20
	commit_all "raise the error budget"

	run env GATE_TIGHTENING_ROOT="$WORK_REPO" BASE_REF="origin/no-such-ref" bash "$SCRIPT"
	[ "$status" -eq 0 ]
	[[ "$output" == *"SKIP"* ]]
}

@test "an undecidable numeric change is reported as unparsed, not as a finding" {
	baseline
	cat >"$WORK_REPO/scripts/check-thing.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
MAX_ERRORS=5
MIN_COVERAGE=80
echo "checking with budget $MAX_ERRORS"
sleep 3
EOF
	commit_all "add an unrelated numeric line"

	run_gate
	[ "$status" -eq 0 ]
	[[ "$output" == *"PASS"* ]]
}
