#!/usr/bin/env bats
# Acceptance surface for scripts/check-doc-skill-refs.sh — backtick-slash skill
# references in doctrine docs (CLAUDE.md, docs/architecture/operating-loop.md,
# skills/SKILL-TIERS.md) must resolve to an existing skills/<dir>. Lines
# carrying a retirement marker (retired|folded|legacy|historical) are exempt.
# Advisory by default (exit 0, prints findings); --strict fails.
#
# Fixtures are generated in tmp trees (not committed) so repo-wide doc scanners
# never see them.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-doc-skill-refs.sh"
    DOCS="$(mktemp -d "$BATS_TMPDIR/docs.XXXXXX")"
    SKILLS="$(mktemp -d "$BATS_TMPDIR/skills.XXXXXX")"
    mkdir -p "$SKILLS/alpha"
}

teardown() {
    [ -n "${DOCS:-}" ] && rm -rf "$DOCS"
    [ -n "${SKILLS:-}" ] && rm -rf "$SKILLS"
}

@test "checker exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "red: phantom skill ref -> strict exits non-zero naming doc and slug" {
    printf 'Run `/zzz-phantom` to do the thing.\n' > "$DOCS/CLAUDE.md"
    run bash "$SCRIPT" --strict --docs-root "$DOCS" --skills-root "$SKILLS"
    [ "$status" -ne 0 ]
    [[ "$output" == *"CLAUDE.md"* ]]
    [[ "$output" == *"zzz-phantom"* ]]
}

@test "red: phantom skill ref -> advisory default exits 0 but still prints the finding" {
    printf 'Run `/zzz-phantom` to do the thing.\n' > "$DOCS/CLAUDE.md"
    run bash "$SCRIPT" --docs-root "$DOCS" --skills-root "$SKILLS"
    [ "$status" -eq 0 ]
    [[ "$output" == *"zzz-phantom"* ]]
    [[ "$output" == *"1 unresolved skill reference(s)"* ]]
}

@test "green: resolving refs (bare and with args) -> strict exits 0" {
    printf 'Run `/alpha` first, then `/alpha --strict` again.\n' > "$DOCS/CLAUDE.md"
    run bash "$SCRIPT" --strict --docs-root "$DOCS" --skills-root "$SKILLS"
    [ "$status" -eq 0 ]
    [[ "$output" == *"0 unresolved skill reference(s)"* ]]
}

@test "exempt: retired-note line citing a gone skill is not flagged" {
    {
        printf '`/zzz-phantom` was retired and folded into `/alpha`.\n'
        printf 'Use `/alpha` going forward.\n'
    } > "$DOCS/CLAUDE.md"
    run bash "$SCRIPT" --strict --docs-root "$DOCS" --skills-root "$SKILLS"
    [ "$status" -eq 0 ]
    [[ "$output" != *"FINDING"* ]]
}

@test "scans the nested doc paths under --docs-root" {
    mkdir -p "$DOCS/docs/architecture" "$DOCS/skills"
    printf 'Skills: `/zzz-loop-phantom` runs the loop.\n' > "$DOCS/docs/architecture/operating-loop.md"
    printf 'Tier 1: `/zzz-tier-phantom`.\n' > "$DOCS/skills/SKILL-TIERS.md"
    run bash "$SCRIPT" --strict --docs-root "$DOCS" --skills-root "$SKILLS"
    [ "$status" -ne 0 ]
    [[ "$output" == *"operating-loop.md"* ]]
    [[ "$output" == *"zzz-loop-phantom"* ]]
    [[ "$output" == *"SKILL-TIERS.md"* ]]
    [[ "$output" == *"zzz-tier-phantom"* ]]
}

@test "non-skill backtick content (paths, plain code) is not matched" {
    {
        printf 'Read `/mnt/c/Users/x` and `docs/templates/intent-issue.md`.\n'
        printf 'Branch `<type>/<bead-id>` and `git -C _beads push` are fine.\n'
    } > "$DOCS/CLAUDE.md"
    run bash "$SCRIPT" --strict --docs-root "$DOCS" --skills-root "$SKILLS"
    [ "$status" -eq 0 ]
    [[ "$output" == *"0 unresolved skill reference(s)"* ]]
}

@test "advisory against the real repo exits 0" {
    run bash "$SCRIPT"
    [ "$status" -eq 0 ]
}

@test "unknown flag exits 2" {
    run bash "$SCRIPT" --bogus
    [ "$status" -eq 2 ]
}

@test "--help exits 0 and documents the check" {
    run bash "$SCRIPT" --help
    [ "$status" -eq 0 ]
    [[ "$output" == *"--strict"* ]]
    [[ "$output" == *"retired"* ]]
}
