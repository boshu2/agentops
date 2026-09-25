#!/usr/bin/env bats
# Acceptance surface for scripts/check-doc-skill-refs.sh — backtick-slash skill
# references and slash-command headings must resolve to an existing
# skills/<dir>. Scope is the pinned non-docs doctrine files (AGENTS.md,
# CLAUDE.md, skills/SKILL-TIERS.md) plus the LIVE docs/** set
# (scripts/lib/docs-scope.sh), gated by a filename-pinned shrink baseline:
#   - a NON-baselined doc with a dead `/skill` ref -> FAIL
#   - a baselined file that no longer offends      -> FAIL (prune it)
# Detection stays slash-syntax + headings ONLY (never bare skill names).
#
# Fixtures are built in tmp trees (--docs-root injected) so repo-wide scanners
# never see them.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-doc-skill-refs.sh"
    DOCS="$BATS_TEST_TMPDIR/docs"
    SKILLS="$BATS_TEST_TMPDIR/skills"
    BASELINE="$BATS_TEST_TMPDIR/baseline"
    : > "$BASELINE"
    mkdir -p "$SKILLS/alpha" "$SKILLS/cc-hooks" "$SKILLS/validate" "$SKILLS/postmortem"
    mkdir -p "$DOCS/docs" "$DOCS/docs/levels" "$DOCS/skills"
}

run_check() {
    run bash "$SCRIPT" --docs-root "$DOCS" --skills-root "$SKILLS" --baseline "$BASELINE"
}

@test "red: phantom skill ref in a pinned non-docs file fails naming doc and slug" {
    printf 'Run `/zzz-phantom` to do the thing.\n' > "$DOCS/CLAUDE.md"
    run_check
    [ "$status" -eq 1 ]
    [[ "$output" == *"NEW-OFFENDER CLAUDE.md"* ]]
    [[ "$output" == *"zzz-phantom"* ]]
}

@test "green: resolving refs (bare and with args) pass" {
    printf 'Run `/alpha` first, then `/alpha --strict` again.\n' > "$DOCS/CLAUDE.md"
    run_check
    [ "$status" -eq 0 ]
    [[ "$output" == *"0 unresolved skill reference(s)"* ]]
}

@test "scans the nested doc paths under --docs-root" {
    mkdir -p "$DOCS/docs/architecture"
    printf '### /zzz-router-phantom\n' > "$DOCS/docs/SKILLS.md"
    printf 'Skills: `/zzz-loop-phantom` runs the loop.\n' > "$DOCS/docs/architecture/rpi-traversal.md"
    printf 'Tier 1: `/zzz-tier-phantom`.\n' > "$DOCS/skills/SKILL-TIERS.md"
    run_check
    [ "$status" -eq 1 ]
    [[ "$output" == *"docs/SKILLS.md"* ]]
    [[ "$output" == *"zzz-router-phantom"* ]]
    [[ "$output" == *"rpi-traversal.md"* ]]
    [[ "$output" == *"zzz-loop-phantom"* ]]
    [[ "$output" == *"SKILL-TIERS.md"* ]]
    [[ "$output" == *"zzz-tier-phantom"* ]]
}

@test "non-skill backtick content (paths, plain code) is not matched" {
    {
        printf 'Read `/mnt/c/Users/x` and `docs/templates/intent-issue.md`.\n'
        printf 'Branch `<type>/<bead-id>` and `git -C _beads push` are fine.\n'
    } > "$DOCS/CLAUDE.md"
    run_check
    [ "$status" -eq 0 ]
    [[ "$output" == *"0 unresolved skill reference(s)"* ]]
}

@test "unknown flag exits 2" {
    run bash "$SCRIPT" --bogus
    [ "$status" -eq 2 ]
}

@test "long-tail doc citing /hooks-authoring fails naming the file and suggesting cc-hooks" {
    # /hooks-authoring is the classic dead ref; cc-hooks is the nearest live skill.
    printf 'Author your own gate with `/hooks-authoring`.\n' > "$DOCS/docs/levels/how-to.md"
    run_check
    [ "$status" -eq 1 ]
    [[ "$output" == *"docs/levels/how-to.md"* ]]
    [[ "$output" == *"did you mean \`/cc-hooks\`?"* ]]
}

@test "SKILL-ROUTER bad ref fails (curated router is checked)" {
    printf '### /zzz-router-phantom\nUse `/alpha` instead.\n' > "$DOCS/docs/SKILL-ROUTER.md"
    run_check
    [ "$status" -eq 1 ]
    [[ "$output" == *"docs/SKILL-ROUTER.md"* ]]
    [[ "$output" == *"zzz-router-phantom"* ]]
}

@test "baselined offender is allowed (no new-offender), gate passes" {
    printf 'Run `/zzz-phantom`.\n' > "$DOCS/docs/legacy-page.md"
    printf 'docs/legacy-page.md\n' > "$BASELINE"
    run_check
    [ "$status" -eq 0 ]
    [[ "$output" != *"NEW-OFFENDER"* ]]
    [[ "$output" != *"FAIL"* ]]
}

@test "stale baseline entry fails demanding a prune" {
    # A clean doc (no dead ref) that is nonetheless listed in the baseline is stale.
    printf 'Use `/alpha` — all good.\n' > "$DOCS/docs/clean-page.md"
    printf 'docs/clean-page.md\n' > "$BASELINE"
    run_check
    [ "$status" -eq 1 ]
    [[ "$output" == *"no longer offend"* ]]
    [[ "$output" == *"docs/clean-page.md"* ]]
}

@test "non-baselined live doc with a dead ref fails as NEW-OFFENDER" {
    printf 'See `/zzz-gone` for details.\n' > "$DOCS/docs/new-page.md"
    run_check
    [ "$status" -eq 1 ]
    [[ "$output" == *"NEW-OFFENDER docs/new-page.md"* ]]
    [[ "$output" == *"zzz-gone"* ]]
}

@test "detection stays slash-only: a bare skill name is never flagged" {
    # Bare `hooks-authoring` (no slash) must NOT trip the gate — false-positive swamp.
    printf 'Author with the `hooks-authoring` skill and read `skills/cc-hooks/`.\n' > "$DOCS/docs/prose.md"
    run_check
    [ "$status" -eq 0 ]
    [[ "$output" != *"NEW-OFFENDER"* ]]
}
