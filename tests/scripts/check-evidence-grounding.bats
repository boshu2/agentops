#!/usr/bin/env bats
#
# Behavioral spec for scripts/check-evidence-grounding.sh — the
# evidence.grounding advisory gate.
#
# The acceptance this pins: an evidence document that cites a repo-relative path
# which does not exist, a full-length commit id git cannot resolve, or a leaked
# scaffold placeholder is FLAGGED; a document whose citations all resolve is
# not. The mechanical half only — whether the evidence actually SUPPORTS the
# claim is a semantic judgment that lives in the validate skill, never here.
#
# Each test builds a throwaway git repo shaped like the real evidence roots and
# points EVIDENCE_GROUNDING_ROOT at it (the script sources scripts/lib/preamble.sh,
# whose REPO_ROOT is anchored at THIS repo — the seam is what lets the spec drive
# a synthetic tree).

setup() {
	REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
	SCRIPT="$REPO_ROOT/scripts/check-evidence-grounding.sh"

	unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX GIT_OBJECT_DIRECTORY GIT_COMMON_DIR GIT_NAMESPACE

	TMP_DIR="$(mktemp -d)"
	WORK_REPO="$TMP_DIR/repo"
	BASELINE="$TMP_DIR/baseline"
	: >"$BASELINE"

	git init -b main "$WORK_REPO" >/dev/null
	git -C "$WORK_REPO" config user.name "Test User"
	git -C "$WORK_REPO" config user.email "test@example.com"
	mkdir -p "$WORK_REPO/docs/audits" "$WORK_REPO/docs/evidence" "$WORK_REPO/docs/handoffs" \
		"$WORK_REPO/scripts" "$WORK_REPO/cli/internal"
	printf '#!/usr/bin/env bash\necho real\n' >"$WORK_REPO/scripts/real.sh"
	printf '# grounded\n\nRan `scripts/real.sh` and it passed.\n' >"$WORK_REPO/docs/evidence/good.md"
}

teardown() {
	rm -rf "$TMP_DIR"
}

commit_all() {
	git -C "$WORK_REPO" add -A
	git -C "$WORK_REPO" commit -q -m "${1:-fixture}"
}

run_gate() {
	run env EVIDENCE_GROUNDING_ROOT="$WORK_REPO" \
		EVIDENCE_GROUNDING_BASELINE="$BASELINE" \
		EVIDENCE_GROUNDING_SHA_CLASS="${SHA_CLASS:-off}" \
		bash "$SCRIPT" "$@"
}

@test "a fully grounded evidence corpus passes" {
	commit_all
	run_gate
	[ "$status" -eq 0 ]
	[[ "$output" == *"PASS"* ]]
}

@test "a cited path that does not exist is flagged, naming doc and path" {
	printf '# receipt\n\nSee `scripts/ghost.sh` for the run.\n' >"$WORK_REPO/docs/evidence/dead-path.md"
	commit_all

	run_gate
	[ "$status" -eq 1 ]
	[[ "$output" == *"docs/evidence/dead-path.md"* ]]
	[[ "$output" == *"scripts/ghost.sh"* ]]
}

@test "a markdown link target that does not exist is flagged" {
	printf '# receipt\n\nSee [the runner](cli/internal/ghost.go).\n' >"$WORK_REPO/docs/evidence/dead-link.md"
	commit_all

	run_gate
	[ "$status" -eq 1 ]
	[[ "$output" == *"cli/internal/ghost.go"* ]]
}

@test "a cited path with a line-number suffix still resolves" {
	printf '# receipt\n\nThe bug is at `scripts/real.sh:12-18`.\n' >"$WORK_REPO/docs/evidence/lineref.md"
	commit_all

	run_gate
	[ "$status" -eq 0 ]
	[[ "$output" == *"PASS"* ]]
}

@test "a leaked scaffold placeholder is flagged" {
	printf '# receipt\n\nOwner: {{owner_name}} signed off.\n' >"$WORK_REPO/docs/evidence/scaffold.md"
	commit_all

	run_gate
	[ "$status" -eq 1 ]
	[[ "$output" == *"scaffold"* ]]
	[[ "$output" == *"docs/evidence/scaffold.md"* ]]
}

@test "a TODO template heading is flagged as a scaffold leak" {
	printf '# receipt\n\n## TODO: fill in the measurement\n' >"$WORK_REPO/docs/evidence/todo.md"
	commit_all

	run_gate
	[ "$status" -eq 1 ]
	[[ "$output" == *"docs/evidence/todo.md"* ]]
}

@test "placeholder syntax QUOTED in a code span is not a scaffold leak" {
	# False-positive guard: an audit that DISCUSSES Go/mustache templates cites
	# `{{.Agent}}` in a code span. That is documentation, not a leak.
	printf '# audit\n\nThe pack renders `{{.Agent}}` at start.\n' >"$WORK_REPO/docs/audits/templates.md"
	commit_all

	run_gate
	[ "$status" -eq 0 ]
	[[ "$output" == *"PASS"* ]]
}

@test "a self-declared HISTORICAL doc is exempt" {
	printf '# old audit\n\n> HISTORICAL — superseded 2026-01-01.\n\nSee `scripts/ghost.sh`.\n' \
		>"$WORK_REPO/docs/audits/old.md"
	commit_all

	run_gate
	[ "$status" -eq 0 ]
	[[ "$output" == *"PASS"* ]]
}

@test "a baselined offender passes and is counted" {
	printf '# receipt\n\nSee `scripts/ghost.sh`.\n' >"$WORK_REPO/docs/evidence/dead-path.md"
	commit_all
	printf '%s\n' 'docs/evidence/dead-path.md  # point-in-time receipt' >"$BASELINE"

	run_gate
	[ "$status" -eq 0 ]
	[[ "$output" == *"PASS"* ]]
	[[ "$output" == *"1 baseline"* ]]
}

@test "a directory baseline entry covers the docs beneath it" {
	mkdir -p "$WORK_REPO/docs/audits/recon-2026-01-01"
	printf '# recon\n\nSee `scripts/ghost.sh`.\n' >"$WORK_REPO/docs/audits/recon-2026-01-01/a.md"
	printf '# recon\n\nSee `cli/internal/ghost.go`.\n' >"$WORK_REPO/docs/audits/recon-2026-01-01/b.md"
	commit_all
	printf '%s\n' 'docs/audits/recon-2026-01-01/  # dated snapshot' >"$BASELINE"

	run_gate
	[ "$status" -eq 0 ]
	[[ "$output" == *"PASS"* ]]
}

@test "a baseline entry that no longer triggers any finding is STALE and fails" {
	# The stale rule needs a COMPLETE offender set, so the sha class must be on
	# (the suppression on partial data is pinned by its own case below).
	commit_all
	printf '%s\n' 'docs/evidence/good.md  # nothing wrong with it any more' >"$BASELINE"

	SHA_CLASS=on run_gate
	[ "$status" -eq 1 ]
	[[ "$output" == *"stale"* || "$output" == *"STALE"* ]]
	[[ "$output" == *"docs/evidence/good.md"* ]]
}

@test "a dead path ADDED on a changed line is not excused by the baseline" {
	# The zero-churn ratchet's teeth: the baseline silences the untouched
	# archive, never a citation written today.
	printf '# receipt\n\nSee `scripts/ghost.sh`.\n' >"$WORK_REPO/docs/evidence/dead-path.md"
	commit_all "baseline commit"
	base="$(git -C "$WORK_REPO" rev-parse HEAD)"
	printf '%s\n' 'docs/evidence/dead-path.md  # point-in-time receipt' >"$BASELINE"

	printf '# receipt\n\nSee `scripts/ghost.sh`.\nAlso `scripts/newly-invented.sh`.\n' \
		>"$WORK_REPO/docs/evidence/dead-path.md"
	commit_all "add a fresh dead citation"

	BASE_REF="$base" run_gate
	[ "$status" -eq 1 ]
	[[ "$output" == *"scripts/newly-invented.sh"* ]]
}

@test "an unresolvable commit id is flagged when the sha class is on" {
	printf '# receipt\n\nLanded at `deadbeefdeadbeefdeadbeefdeadbeefdeadbeef`.\n' \
		>"$WORK_REPO/docs/evidence/sha.md"
	commit_all

	SHA_CLASS=on run_gate
	[ "$status" -eq 1 ]
	[[ "$output" == *"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"* ]]
}

@test "a resolvable commit id is not flagged" {
	commit_all
	sha="$(git -C "$WORK_REPO" rev-parse HEAD)"
	printf '# receipt\n\nLanded at `%s`.\n' "$sha" >"$WORK_REPO/docs/evidence/sha.md"
	commit_all "cite the real sha"

	SHA_CLASS=on run_gate
	[ "$status" -eq 0 ]
	[[ "$output" == *"PASS"* ]]
}

@test "with the sha class off the gate says so and suppresses the stale rule" {
	# Never prune a baseline on an incomplete offender set.
	commit_all
	printf '%s\n' 'docs/evidence/good.md  # would look stale on partial data' >"$BASELINE"

	SHA_CLASS=off run_gate
	[ "$status" -eq 0 ]
	[[ "$output" == *"UNCHECKED"* ]]
	[[ "$output" == *"stale"* ]]
}

@test "a citation into gitignored runtime state is not a dead path" {
	# ADR-0016: `.agents/**` is disposable local state. A clean checkout is
	# SUPPOSED not to have it, so citing one is documentation, not a break.
	printf '/.agents/*\n' >"$WORK_REPO/.gitignore"
	printf '# receipt\n\nHandoff written to `.agents/ao/handoff/x.json`.\n' \
		>"$WORK_REPO/docs/evidence/runtime.md"
	commit_all

	run_gate
	[ "$status" -eq 0 ]
	[[ "$output" == *"PASS"* ]]
}

@test "docs outside the three evidence roots are never scanned" {
	printf '# guide\n\nSee `scripts/ghost.sh`.\n' >"$WORK_REPO/docs/guide.md"
	commit_all

	run_gate
	[ "$status" -eq 0 ]
	[[ "$output" == *"PASS"* ]]
}

@test "an untracked evidence doc is not scanned (repo-tracked scope)" {
	commit_all
	printf '# scratch\n\nSee `scripts/ghost.sh`.\n' >"$WORK_REPO/docs/evidence/scratch.md"

	run_gate
	[ "$status" -eq 0 ]
	[[ "$output" == *"PASS"* ]]
}
