#!/usr/bin/env bats
# Acceptance surface for ag-f0i (advisory slice): audit skill-metadata
# `context_rel[].with` resolution. The skill-frontmatter.v2 schema declares
# context_rel.with as a *skill slug*, so every `with:` value MUST name an
# existing peer skill directory. No existing validator checks resolution —
# validate-skill-frontmatter.sh only checks presence/shape. This auditor closes
# that gap. Advisory by default (exit 0, logs findings); --strict fails;
# --json emits a machine-readable verdict naming the offending field + value.
#
# Fixtures are generated in a tmp tree (not committed) so repo-wide SKILL.md
# scanners never see them.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/audit-skill-metadata.sh"
    ROOT="$(mktemp -d)"
}

teardown() {
    [ -n "${ROOT:-}" ] && rm -rf "$ROOT"
}

# mkskill <root> <name> [with-target]
# Writes a minimal valid SKILL.md frontmatter; adds a context_rel.with edge
# when a target is supplied.
mkskill() {
    local root="$1" name="$2" with="${3:-}"
    mkdir -p "$root/$name"
    {
        echo "---"
        echo "name: $name"
        echo "description: fixture skill $name"
        echo "hexagonal_role: supporting"
        echo "practices:"
        echo "- tdd"
        if [ -n "$with" ]; then
            echo "context_rel:"
            echo "- kind: customer-of"
            echo "  with: $with"
        fi
        echo "---"
        echo "# $name"
    } > "$root/$name/SKILL.md"
}

@test "auditor exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "all context_rel.with resolve -> advisory and strict both pass" {
    mkskill "$ROOT" alpha beta
    mkskill "$ROOT" beta
    run bash "$SCRIPT" --skills-root "$ROOT"
    [ "$status" -eq 0 ]
    run bash "$SCRIPT" --strict --skills-root "$ROOT"
    [ "$status" -eq 0 ]
}

@test "unresolved context_rel.with -> advisory exits 0 and logs the finding" {
    mkskill "$ROOT" alpha ghost
    run bash "$SCRIPT" --skills-root "$ROOT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"ghost"* ]]
}

@test "unresolved context_rel.with -> strict exits non-zero naming file+field+value" {
    mkskill "$ROOT" alpha ghost
    run bash "$SCRIPT" --strict --skills-root "$ROOT"
    [ "$status" -ne 0 ]
    [[ "$output" == *"alpha/SKILL.md"* ]]
    [[ "$output" == *"context_rel"* ]]
    [[ "$output" == *"ghost"* ]]
}

@test "--json emits a valid verdict naming the offending field and value" {
    mkskill "$ROOT" alpha ghost
    mkskill "$ROOT" beta
    run bash "$SCRIPT" --json --skills-root "$ROOT"
    [ "$status" -eq 0 ]
    echo "$output" | jq empty
    [ "$(echo "$output" | jq -r '.valid')" = "false" ]
    [ "$(echo "$output" | jq -r '.findings[0].field')" = "context_rel.with" ]
    [ "$(echo "$output" | jq -r '.findings[0].value')" = "ghost" ]
}

@test "--json on a clean tree reports valid:true with no findings" {
    mkskill "$ROOT" alpha beta
    mkskill "$ROOT" beta
    run bash "$SCRIPT" --json --skills-root "$ROOT"
    [ "$status" -eq 0 ]
    [ "$(echo "$output" | jq -r '.valid')" = "true" ]
    [ "$(echo "$output" | jq -r '.findings | length')" = "0" ]
}

@test "SKILL_METADATA_SKILLS_ROOT env selects the tree" {
    mkskill "$ROOT" alpha ghost
    export SKILL_METADATA_SKILLS_ROOT="$ROOT"
    run bash "$SCRIPT" --strict
    unset SKILL_METADATA_SKILLS_ROOT
    [ "$status" -ne 0 ]
    [[ "$output" == *"ghost"* ]]
}

@test "unknown flag exits 2" {
    run bash "$SCRIPT" --bogus
    [ "$status" -eq 2 ]
}

@test "--help exits 0 and documents the check" {
    run bash "$SCRIPT" --help
    [ "$status" -eq 0 ]
    [[ "$output" == *"context_rel"* ]]
}
