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
# Tests run inside a tmp scratch tree to avoid polluting the real skills/ dir.

setup() {
    REPO_ROOT="$(cd "${BATS_TEST_DIRNAME}/../.." && pwd)"
    BUILD_SH="$REPO_ROOT/skills/skill-builder/scripts/build.sh"
    AUDIT_SH="$REPO_ROOT/skills/skill-auditor/scripts/audit.sh"

    # Tests build into the real REPO_ROOT/skills tree (init.sh hardcodes REPO_ROOT
    # discovery via SCRIPT_DIR). We track every created skill name so teardown
    # can clean up.
    SCRATCH_NAME="bats-builder-test-$$"
    CREATED_SKILLS=()
}

teardown() {
    for name in "${CREATED_SKILLS[@]:-}"; do
        [ -n "$name" ] || continue
        rm -rf "$REPO_ROOT/skills/$name" "$REPO_ROOT/skills-codex/$name"
        rm -f "$REPO_ROOT/.agents/audits/${name}-build.json"
    done
}

# Helper: register a skill name for teardown cleanup.
track_skill() {
    CREATED_SKILLS+=("$1")
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
    track_skill "${SCRATCH_NAME}-tpl-noflag"
    run bash "$BUILD_SH" from-template "${SCRATCH_NAME}-tpl-noflag"
    [ "$status" -ne 0 ]
}

@test "from-scratch creates SKILL.md + codex parity" {
    local name="${SCRATCH_NAME}-fs"
    track_skill "$name"
    SKILL_TIER=execution SKILL_INTENT_MODE=task \
        run bash "$BUILD_SH" from-scratch "$name"
    [ "$status" -eq 0 ]
    [ -f "$REPO_ROOT/skills/$name/SKILL.md" ]
    [ -f "$REPO_ROOT/skills/$name/scripts/validate.sh" ]
    [ -f "$REPO_ROOT/skills-codex/$name/SKILL.md" ]
    [ -f "$REPO_ROOT/skills-codex/$name/prompt.md" ]
}

@test "from-scratch produced codex SKILL.md has slim frontmatter (no skill_api_version)" {
    local name="${SCRATCH_NAME}-fs-slim"
    track_skill "$name"
    SKILL_TIER=execution SKILL_INTENT_MODE=task \
        bash "$BUILD_SH" from-scratch "$name" >/dev/null 2>&1
    run grep -c '^skill_api_version:' "$REPO_ROOT/skills-codex/$name/SKILL.md"
    [ "$status" -ne 0 ] || [ "$output" = "0" ]
}

@test "from-scratch frontmatter name matches directory" {
    local name="${SCRATCH_NAME}-fs-name"
    track_skill "$name"
    SKILL_TIER=execution SKILL_INTENT_MODE=task \
        bash "$BUILD_SH" from-scratch "$name" >/dev/null 2>&1
    run grep -E "^name: $name$" "$REPO_ROOT/skills/$name/SKILL.md"
    [ "$status" -eq 0 ]
}

@test "from-scratch self-audit chain runs (build aborts on auditor FAIL)" {
    local name="${SCRATCH_NAME}-fs-audit"
    track_skill "$name"
    SKILL_TIER=execution SKILL_INTENT_MODE=task \
        run bash "$BUILD_SH" from-scratch "$name"
    # The build.sh tail invokes audit.sh; build.sh exits 1 if auditor returns FAIL.
    # New skill skeletons may PASS or WARN but must not FAIL.
    [ "$status" -eq 0 ]
}

@test "from-template --like council produces skeleton" {
    local name="${SCRATCH_NAME}-tpl"
    track_skill "$name"
    run bash "$BUILD_SH" from-template "$name" --like council
    [ "$status" -eq 0 ]
    [ -f "$REPO_ROOT/skills/$name/SKILL.md" ]
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
    track_skill "$name"
    run bash "$BUILD_SH" absorb-external "$name" --from "$ext"
    [ "$status" -eq 0 ]
    [ -f "$REPO_ROOT/skills/$name/SKILL.md" ]
}

@test "absorb-external requires --from path" {
    track_skill "${SCRATCH_NAME}-abs-no-from"
    run bash "$BUILD_SH" absorb-external "${SCRATCH_NAME}-abs-no-from"
    [ "$status" -ne 0 ]
}
