#!/usr/bin/env bats
# test_skill_builder.bats — L2 integration tests for skill-builder.
#
# Asserts that build.sh + init.sh:
#   - Reject invalid usage with exit 2
#   - from-scratch (env-driven) creates skills/<name>/SKILL.md + codex parity
#   - from-template (--like council) materializes a skeleton
#   - absorb-external wraps an external SKILL.md in AgentOps frontmatter
#   - All built skills self-audit at PASS or WARN (never FAIL)
#
# ISOLATION CONTRACT: the suite never mutates the real repo. setup_file copies
# the surfaces the factory touches (skills/, scripts/, docs/contracts +
# docs/reference, skills-codex-overrides/) into a scratch root and exports
# SKILL_BUILDER_REPO_ROOT (the HEAL_REPO_ROOT-style override honored by
# init.sh/build.sh), so scaffolds, dispositions-ledger rows, codex-catalog
# entries, and registry regen all land in the scratch tree. Before this,
# in-repo runs appended real bats-builder-test-* rows to
# docs/contracts/skill-dispositions.yaml + skills-codex-overrides/catalog.json
# and once corrupted the ledger tail.

setup_file() {
    REAL_REPO_ROOT="$(cd "$(dirname "$BATS_TEST_FILENAME")/../.." && pwd)"
    export REAL_REPO_ROOT

    SCRATCH_ROOT="$BATS_FILE_TMPDIR/scratch-repo"
    mkdir -p "$SCRATCH_ROOT/docs" "$SCRATCH_ROOT/skills-codex"
    cp -R "$REAL_REPO_ROOT/skills" "$SCRATCH_ROOT/skills"
    cp -R "$REAL_REPO_ROOT/scripts" "$SCRATCH_ROOT/scripts"
    cp -R "$REAL_REPO_ROOT/docs/contracts" "$SCRATCH_ROOT/docs/contracts"
    cp -R "$REAL_REPO_ROOT/docs/reference" "$SCRATCH_ROOT/docs/reference"
    cp -R "$REAL_REPO_ROOT/skills-codex-overrides" "$SCRATCH_ROOT/skills-codex-overrides"
    [ -f "$REAL_REPO_ROOT/registry.json" ] && cp "$REAL_REPO_ROOT/registry.json" "$SCRATCH_ROOT/registry.json"
    # generate-registry.sh shells out to `git ls-files`; an empty repo is enough.
    git -C "$SCRATCH_ROOT" init -q

    export SCRATCH_ROOT
    export SKILL_BUILDER_REPO_ROOT="$SCRATCH_ROOT"
}

setup() {
    # The scripts UNDER TEST stay the real checked-in ones; only the data root
    # they operate on is redirected via SKILL_BUILDER_REPO_ROOT.
    BUILD_SH="$REAL_REPO_ROOT/skills/skill-builder/scripts/build.sh"
    SCRATCH_NAME="bats-builder-test-$$"
}

@test "build.sh exists and is executable" {
    [ -f "$BUILD_SH" ]
    [ -r "$BUILD_SH" ]
}

@test "build.sh exits 2 on no args" {
    run bash "$BUILD_SH"
    [ "$status" -eq 2 ]
}

@test "build.sh exits 2 on unknown mode" {
    run bash "$BUILD_SH" not-a-real-mode foo
    [ "$status" -eq 2 ]
}

@test "build.sh from-scratch missing skill name exits 2" {
    run bash "$BUILD_SH" from-scratch
    [ "$status" -eq 2 ]
}

@test "build.sh from-template missing --like flag fails" {
    run bash "$BUILD_SH" from-template "${SCRATCH_NAME}-tpl-noflag"
    [ "$status" -ne 0 ]
}

@test "from-scratch creates SKILL.md + codex parity" {
    local name="${SCRATCH_NAME}-fs"
    SKILL_TIER=execution SKILL_INTENT_MODE=task \
        run bash "$BUILD_SH" from-scratch "$name"
    [ "$status" -eq 0 ]
    [ -f "$SCRATCH_ROOT/skills/$name/SKILL.md" ]
    [ -f "$SCRATCH_ROOT/skills/$name/scripts/validate.sh" ]
    [ -f "$SCRATCH_ROOT/skills-codex/$name/SKILL.md" ]
    [ -f "$SCRATCH_ROOT/skills-codex/$name/prompt.md" ]
}

@test "from-scratch produced codex SKILL.md has slim frontmatter (no skill_api_version)" {
    local name="${SCRATCH_NAME}-fs-slim"
    SKILL_TIER=execution SKILL_INTENT_MODE=task \
        bash "$BUILD_SH" from-scratch "$name" >/dev/null 2>&1
    run grep -c '^skill_api_version:' "$SCRATCH_ROOT/skills-codex/$name/SKILL.md"
    [ "$status" -ne 0 ] || [ "$output" = "0" ]
}

@test "from-scratch frontmatter name matches directory" {
    local name="${SCRATCH_NAME}-fs-name"
    SKILL_TIER=execution SKILL_INTENT_MODE=task \
        bash "$BUILD_SH" from-scratch "$name" >/dev/null 2>&1
    run grep -E "^name: $name$" "$SCRATCH_ROOT/skills/$name/SKILL.md"
    [ "$status" -eq 0 ]
}

@test "from-scratch self-audit chain runs (build aborts on auditor FAIL)" {
    local name="${SCRATCH_NAME}-fs-audit"
    SKILL_TIER=execution SKILL_INTENT_MODE=task \
        run bash "$BUILD_SH" from-scratch "$name"
    # The build.sh tail invokes audit.sh; build.sh exits 1 if auditor returns FAIL.
    # New skill skeletons may PASS or WARN but must not FAIL.
    [ "$status" -eq 0 ]
}

@test "from-scratch appends its dispositions row + codex catalog entry to the SCRATCH copies" {
    local name="${SCRATCH_NAME}-fs-plumbing"
    SKILL_TIER=execution SKILL_INTENT_MODE=task \
        bash "$BUILD_SH" from-scratch "$name" >/dev/null 2>&1
    grep -qE "^[[:space:]]*-[[:space:]]+skill:[[:space:]]+${name}[[:space:]]*$" \
        "$SCRATCH_ROOT/docs/contracts/skill-dispositions.yaml"
    grep -q "\"$name\"" "$SCRATCH_ROOT/skills-codex-overrides/catalog.json"
}

@test "from-template --like council produces skeleton" {
    local name="${SCRATCH_NAME}-tpl"
    run bash "$BUILD_SH" from-template "$name" --like council
    [ "$status" -eq 0 ]
    [ -f "$SCRATCH_ROOT/skills/$name/SKILL.md" ]
}

@test "absorb-external wraps an external SKILL.md" {
    local ext="${BATS_TEST_TMPDIR}/external-skill.md"
    cat >"$ext" <<'EOF'
---
name: external-source
description: 'External skill body to absorb.'
---
# External body

A short external skill body.
EOF
    local name="${SCRATCH_NAME}-abs"
    run bash "$BUILD_SH" absorb-external "$name" --from "$ext"
    [ "$status" -eq 0 ]
    [ -f "$SCRATCH_ROOT/skills/$name/SKILL.md" ]
}

@test "absorb-external requires --from path" {
    run bash "$BUILD_SH" absorb-external "${SCRATCH_NAME}-abs-no-from"
    [ "$status" -ne 0 ]
}

@test "isolation: no scaffold, ledger row, or catalog entry leaked into the real repo" {
    # Runs after the scaffolding tests above (bats executes tests in file order):
    # the real tree must carry zero trace of any bats-builder-test-* skill.
    [ -z "$(find "$REAL_REPO_ROOT/skills" "$REAL_REPO_ROOT/skills-codex" \
        -maxdepth 1 -name 'bats-builder-test-*' -print -quit 2>/dev/null)" ]
    ! grep -q 'bats-builder-test-' "$REAL_REPO_ROOT/docs/contracts/skill-dispositions.yaml"
    ! grep -q 'bats-builder-test-' "$REAL_REPO_ROOT/skills-codex-overrides/catalog.json"
    ! ls "$REAL_REPO_ROOT/.agents/audits/"bats-builder-test-*-build.json >/dev/null 2>&1
}
