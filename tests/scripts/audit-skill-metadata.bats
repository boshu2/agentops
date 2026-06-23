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

@test "red-then-green: zzz-phantom edge fails strict naming source and target, clean root passes" {
    # RED: a temp skills root whose only skill cites a phantom peer must fail
    # --strict and name BOTH ends of the dangling edge.
    mkskill "$ROOT" srcskill zzz-phantom
    run bash "$SCRIPT" --strict --skills-root "$ROOT"
    [ "$status" -ne 0 ]
    [[ "$output" == *"srcskill/SKILL.md"* ]]
    [[ "$output" == *"zzz-phantom"* ]]

    # GREEN: a clean temp root (every context_rel.with resolves) exits 0.
    GREEN="$(mktemp -d "$BATS_TMPDIR/clean-skills.XXXXXX")"
    mkskill "$GREEN" alpha beta
    mkskill "$GREEN" beta
    run bash "$SCRIPT" --strict --skills-root "$GREEN"
    rm -rf "$GREEN"
    [ "$status" -eq 0 ]
    [[ "$output" == *"0 unresolved context_rel.with edge(s)"* ]]
}

@test "baseline pin: real skills root reports the current unresolved-edge count (advisory contract)" {
    # ── BASELINE-PIN ─────────────────────────────────────────────────────────
    # The auditor is ADVISORY by default: it must exit 0 against the real
    # skills/ tree even while known dangling context_rel.with edges exist. This
    # case pins the exact count so any drift — new rot OR the drain landing —
    # surfaces here deliberately.
    #
    # 2026-06-23 drain update (verified, not a blind bump): the original 5
    # session-bootstrap AGENTS-*.md doc-as-slug edges have all drained (0 AGENTS
    # edges remain). The 2 that remain are verified-legitimate FOLDED-TRIGGER
    # edges from the ag-s43tg skill-corpus prune — names that route to an
    # absorbing skill but are not themselves skill DIRECTORIES:
    #   - codex-exec       with: codex-sandbox-evidence  (folded INTO codex-exec)
    #   - workflow-builder with: operating-loop-workflow  (folded INTO rpi)
    # Neither ever existed as a skill dir; both are intentional self/cross-refs
    # to absorbed triggers, so they are tolerated known-danglers, not rot.
    #
    # >>> WHEN THE FOLDED-TRIGGER EDGES ARE REPOINTED/REMOVED: update to 0 <<<
    # (at which point this becomes the clean-tree pin).
    EXPECTED_UNRESOLVED=2
    run bash "$SCRIPT" --skills-root "$REPO_ROOT/skills"
    [ "$status" -eq 0 ]
    # Anchor the count with its stable preceding text ("checked, ") so the glob
    # cannot match on a LEADING digit: a bare *"2 unresolved..."* substring is
    # also contained in "12 unresolved...", so 10 new rot edges (count 12) would
    # fail open. "checked, 2 unresolved" does not appear in "checked, 12 unresolved".
    [[ "$output" == *"checked, ${EXPECTED_UNRESOLVED} unresolved context_rel.with edge(s)"* ]]

    # Pin the EXACT tolerated edges by IDENTITY, not just the count: a count-only
    # pin would pass even if a NEW dangling edge replaced one folded-trigger edge
    # (count stays 2), masking fresh rot. Assert both known folded-trigger
    # danglers are the ones present, so any substitution surfaces here too.
    [[ "$output" == *"codex-exec/SKILL.md: context_rel.with -> 'codex-sandbox-evidence'"* ]]
    [[ "$output" == *"workflow-builder/SKILL.md: context_rel.with -> 'operating-loop-workflow'"* ]]
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
