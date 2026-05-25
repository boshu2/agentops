#!/usr/bin/env bats
#
# Tests for the skill-body command-ref gate (ag-4x8):
# scripts/validate-skill-body-refs.sh. The gate scans the inline-code prose of
# SKILL.md + references/*.md for `ao <command>`/`ao <command> --flag` tokens and
# validates them against the live `ao` help tree — the complement of the
# fenced-snippet gate.
#
# Fixture-driven: AGENTOPS_SKILL_BODY_ROOTS points the gate at a throwaway tree
# so we never mutate tracked skills. The final case runs the gate against the
# real committed tree to assert main stays green.
#
# Heredocs are quoted (<<'PY' is not used here, but fixture writes use cat) to
# keep `ao` refs literal. AO_BIN + REPO_ROOT reach the script via the env.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    export REPO_ROOT
}

# require_ao echoes a path to a usable `ao` binary, or skips. The authoritative
# full check is the validate-skill-body-refs CI job (which always builds ao);
# this bats job has no Go setup, so ao-dependent cases skip when toolchain absent.
require_ao() {
    if [[ -x "$REPO_ROOT/cli/bin/ao" ]]; then
        echo "$REPO_ROOT/cli/bin/ao"
        return
    fi
    if command -v go >/dev/null 2>&1; then
        local bin="$BATS_TEST_TMPDIR/ao"
        if ( cd "$REPO_ROOT/cli" && go build -o "$bin" ./cmd/ao ) >/dev/null 2>&1; then
            echo "$bin"
            return
        fi
    fi
    skip "no ao binary and no Go toolchain to build one (covered by validate-skill-body-refs CI job)"
}

# write_skill <fixture-root> <skill-name> <body-line>
write_skill() {
    local root="$1" name="$2" body="$3"
    mkdir -p "$root/$name"
    {
        printf -- '---\n'
        printf 'name: %s\n' "$name"
        printf -- '---\n'
        printf '# %s\n\n' "$name"
        printf '%s\n' "$body"
    } > "$root/$name/SKILL.md"
}

@test "passes when every inline-code prose ref resolves against the live CLI" {
    AO_BIN="$(require_ao)"
    local fixture="$BATS_TEST_TMPDIR/clean"
    write_skill "$fixture" "good" 'Run \`ao lookup --query "topic"\` then \`ao goals measure\`.'

    run env AGENTOPS_AO_BIN="$AO_BIN" AGENTOPS_SKILL_BODY_ROOTS="$fixture" \
        bash "$REPO_ROOT/scripts/validate-skill-body-refs.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"validation passed"* ]]
}

@test "fails on an injected stale command ref in prose" {
    AO_BIN="$(require_ao)"
    local fixture="$BATS_TEST_TMPDIR/bad-cmd"
    write_skill "$fixture" "stale" 'Submit nightly work to \`ao schedule\` via the daemon.'

    run env AGENTOPS_AO_BIN="$AO_BIN" AGENTOPS_SKILL_BODY_ROOTS="$fixture" \
        bash "$REPO_ROOT/scripts/validate-skill-body-refs.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *"unknown ao command"* ]]
    [[ "$output" == *"ao schedule"* ]]
}

@test "fails on an injected stale flag ref in prose" {
    AO_BIN="$(require_ao)"
    local fixture="$BATS_TEST_TMPDIR/bad-flag"
    write_skill "$fixture" "staleflag" 'Use \`ao inject --bogus-flag\` to load context.'

    run env AGENTOPS_AO_BIN="$AO_BIN" AGENTOPS_SKILL_BODY_ROOTS="$fixture" \
        bash "$REPO_ROOT/scripts/validate-skill-body-refs.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *"flag --bogus-flag not found"* ]]
}

@test "skips a stale ref on a line bearing a historical marker" {
    AO_BIN="$(require_ao)"
    local fixture="$BATS_TEST_TMPDIR/historical"
    write_skill "$fixture" "removed" 'The \`ao schedule\` command was removed in 3.0.'

    run env AGENTOPS_AO_BIN="$AO_BIN" AGENTOPS_SKILL_BODY_ROOTS="$fixture" \
        bash "$REPO_ROOT/scripts/validate-skill-body-refs.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"validation passed"* ]]
}

@test "ignores bare-prose English that merely follows the word ao" {
    AO_BIN="$(require_ao)"
    local fixture="$BATS_TEST_TMPDIR/prose"
    # Bare prose (no backticks) must never be treated as a command invocation.
    write_skill "$fixture" "prose" 'First, search and inject existing knowledge (if ao available).'

    run env AGENTOPS_AO_BIN="$AO_BIN" AGENTOPS_SKILL_BODY_ROOTS="$fixture" \
        bash "$REPO_ROOT/scripts/validate-skill-body-refs.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"validation passed"* ]]
}

@test "the committed skill+codex tree passes the full gate" {
    AO_BIN="$(require_ao)"
    run env AGENTOPS_AO_BIN="$AO_BIN" bash "$REPO_ROOT/scripts/validate-skill-body-refs.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"validation passed"* ]]
}
