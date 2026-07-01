#!/usr/bin/env bats
# Acceptance surface for scripts/check-doc-skill-refs.sh --all-docs.
#
# --all-docs widens the scan from the fixed doctrine files to the union of them
# PLUS the LIVE docs/** set (scripts/lib/docs-scope.sh), gated by a
# filename-pinned shrink baseline. The ratchet is two-way:
#   - a NON-baselined live doc with a dead `/skill` ref -> FAIL
#   - a baselined file that no longer offends           -> FAIL (prune it)
# Detection stays slash-syntax + headings ONLY (never bare skill names).
#
# Fixtures are built in tmp trees (DOCS_ROOT-injected) so repo-wide scanners
# never see them. The default-mode byte-identical guarantee is asserted at the
# fixture level (adding --all-docs must not perturb the no-flag output).

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-doc-skill-refs.sh"
    DOCS="$(mktemp -d "$BATS_TMPDIR/docs.XXXXXX")"
    SKILLS="$(mktemp -d "$BATS_TMPDIR/skills.XXXXXX")"
    BASELINE="$(mktemp "$BATS_TMPDIR/baseline.XXXXXX")"
    mkdir -p "$SKILLS/alpha" "$SKILLS/cc-hooks" "$SKILLS/validate" "$SKILLS/post-mortem"
    mkdir -p "$DOCS/docs" "$DOCS/docs/levels" "$DOCS/skills"
    # docs-scope.sh emits paths anchored at DOCS_ROOT; a live docs/** file must
    # exist for --all-docs to pick it up.
}

# doc_findings_json is intentionally omitted — the checker is line-based text.

@test "the all-docs bats twin points at a real script" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "long-tail doc citing /hooks-authoring fails naming the file and suggesting cc-hooks" {
    # /hooks-authoring is the classic dead ref; cc-hooks is the nearest live skill.
    printf 'Author your own gate with `/hooks-authoring`.\n' > "$DOCS/docs/levels/how-to.md"
    run bash "$SCRIPT" --all-docs --strict --docs-root "$DOCS" --skills-root "$SKILLS" --baseline "$BASELINE"
    [ "$status" -ne 0 ]
    [[ "$output" == *"docs/levels/how-to.md"* ]]
    [[ "$output" == *"hooks-authoring"* ]]
    # nearest-live-skill suggestion (simple prefix/substring match): cc-hooks.
    [[ "$output" == *"cc-hooks"* ]]
}

@test "SKILL-ROUTER bad ref fails in --all-docs --strict (curated router is checked)" {
    printf '### /zzz-router-phantom\nUse `/alpha` instead.\n' > "$DOCS/docs/SKILL-ROUTER.md"
    run bash "$SCRIPT" --all-docs --strict --docs-root "$DOCS" --skills-root "$SKILLS" --baseline "$BASELINE"
    [ "$status" -ne 0 ]
    [[ "$output" == *"docs/SKILL-ROUTER.md"* ]]
    [[ "$output" == *"zzz-router-phantom"* ]]
}

@test "retired-exemption line passes even under --all-docs" {
    printf '`/hooks-authoring` was retired; use `/cc-hooks` now.\n' > "$DOCS/docs/history.md"
    run bash "$SCRIPT" --all-docs --strict --docs-root "$DOCS" --skills-root "$SKILLS" --baseline "$BASELINE"
    [ "$status" -eq 0 ]
    [[ "$output" != *"NEW-OFFENDER"* ]]
}

@test "baselined offender is allowed (no new-offender), gate passes" {
    printf 'Run `/zzz-phantom`.\n' > "$DOCS/docs/legacy-page.md"
    printf 'docs/legacy-page.md\n' > "$BASELINE"
    run bash "$SCRIPT" --all-docs --strict --docs-root "$DOCS" --skills-root "$SKILLS" --baseline "$BASELINE"
    [ "$status" -eq 0 ]
    [[ "$output" != *"NEW-OFFENDER"* ]]
    [[ "$output" != *"FAIL"* ]]
}

@test "stale baseline entry fails demanding a prune" {
    # A clean doc (no dead ref) that is nonetheless listed in the baseline is stale.
    printf 'Use `/alpha` — all good.\n' > "$DOCS/docs/clean-page.md"
    printf 'docs/clean-page.md\n' > "$BASELINE"
    run bash "$SCRIPT" --all-docs --strict --docs-root "$DOCS" --skills-root "$SKILLS" --baseline "$BASELINE"
    [ "$status" -ne 0 ]
    [[ "$output" == *"no longer offend"* ]]
    [[ "$output" == *"docs/clean-page.md"* ]]
}

@test "non-baselined live doc with a dead ref fails as NEW-OFFENDER" {
    printf 'See `/zzz-gone` for details.\n' > "$DOCS/docs/new-page.md"
    run bash "$SCRIPT" --all-docs --strict --docs-root "$DOCS" --skills-root "$SKILLS" --baseline "$BASELINE"
    [ "$status" -ne 0 ]
    [[ "$output" == *"NEW-OFFENDER"* ]]
    [[ "$output" == *"docs/new-page.md"* ]]
    [[ "$output" == *"zzz-gone"* ]]
}

@test "default mode is byte-identical whether or not --all-docs exists (fixture level)" {
    # A single pinned doc with a resolving ref. Default mode must report exactly
    # the pinned-set scan, unaffected by the presence of live docs/** files.
    printf 'Run `/alpha`.\n' > "$DOCS/CLAUDE.md"
    printf 'Live doc with a phantom `/zzz-only-live`.\n' > "$DOCS/docs/only-live.md"
    # default mode (no --all-docs): scans pinned set only, ignores docs/only-live.md
    run bash "$SCRIPT" --strict --docs-root "$DOCS" --skills-root "$SKILLS"
    [ "$status" -eq 0 ]
    [[ "$output" == *"0 unresolved skill reference(s)"* ]]
    [[ "$output" != *"only-live"* ]]
    [[ "$output" != *"zzz-only-live"* ]]
    # --all-docs sees the live doc and (unbaselined) fails on it
    run bash "$SCRIPT" --all-docs --strict --docs-root "$DOCS" --skills-root "$SKILLS" --baseline "$BASELINE"
    [ "$status" -ne 0 ]
    [[ "$output" == *"only-live"* ]]
}

@test "detection stays slash-only: a bare skill name is never flagged under --all-docs" {
    # Bare `hooks-authoring` (no slash) must NOT trip the gate — false-positive swamp.
    printf 'Author with the `hooks-authoring` skill and read `skills/cc-hooks/`.\n' > "$DOCS/docs/prose.md"
    run bash "$SCRIPT" --all-docs --strict --docs-root "$DOCS" --skills-root "$SKILLS" --baseline "$BASELINE"
    [ "$status" -eq 0 ]
    [[ "$output" != *"NEW-OFFENDER"* ]]
}

@test "--all-docs against the real repo passes strict with the checked-in baseline" {
    run bash "$SCRIPT" --all-docs --strict
    [ "$status" -eq 0 ]
}
