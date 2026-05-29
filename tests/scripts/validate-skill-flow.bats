#!/usr/bin/env bats
# Acceptance surface for scripts/validate-skill-flow.sh — the skill-flow
# connectivity gate (follow-up to audit-skill-metadata.sh, which deferred the
# `consumes` vocabulary and connectivity checks).
#
# Contract: docs/contracts/skill-flow.md
# Fixtures live in a tmp tree so the repo-wide SKILL.md scanners never see them.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/validate-skill-flow.sh"
    ROOT="$(mktemp -d)"
    ALLOW="$(mktemp)"
    : > "$ALLOW"   # empty allowlist by default
}

teardown() {
    [ -n "${ROOT:-}" ] && rm -rf "$ROOT"
    [ -n "${ALLOW:-}" ] && rm -f "$ALLOW"
}

# mkskill <root> <name> [consumes_csv] [produces_csv] [ctx_with] [mdeps_csv]
# Writes a minimal valid SKILL.md frontmatter. CSV args are comma-separated.
mkskill() {
    local root="$1" name="$2" consumes="${3:-}" produces="${4:-}" ctx="${5:-}" mdeps="${6:-}"
    mkdir -p "$root/$name"
    {
        echo "---"
        echo "name: $name"
        echo "description: fixture skill $name"
        echo "hexagonal_role: supporting"
        echo "practices:"
        echo "- tdd"
        if [ -n "$consumes" ]; then
            echo "consumes:"
            IFS=',' read -ra items <<< "$consumes"
            for i in "${items[@]}"; do echo "- $i"; done
        fi
        if [ -n "$produces" ]; then
            echo "produces:"
            IFS=',' read -ra items <<< "$produces"
            for i in "${items[@]}"; do echo "- $i"; done
        fi
        if [ -n "$ctx" ]; then
            echo "context_rel:"
            echo "- kind: customer-of"
            echo "  with: $ctx"
        fi
        if [ -n "$mdeps" ]; then
            echo "metadata:"
            echo "  dependencies:"
            IFS=',' read -ra items <<< "$mdeps"
            for i in "${items[@]}"; do echo "  - $i"; done
        fi
        echo "---"
        echo "# $name"
    } > "$root/$name/SKILL.md"
}

run_gate() { run bash "$SCRIPT" --skills-root "$ROOT" --allowlist "$ALLOW" "$@"; }

@test "gate exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "two skills connected via context_rel -> PASS" {
    mkskill "$ROOT" alpha "" "" beta ""
    mkskill "$ROOT" beta
    # beta is referenced by alpha so it is connected; alpha references beta.
    run_gate
    [ "$status" -eq 0 ]
    [[ "$output" == *"skill flow is connected"* ]]
}

@test "consumes a whitelisted external input -> PASS (not an orphan target failure)" {
    mkskill "$ROOT" alpha "repo-context" "" beta ""
    mkskill "$ROOT" beta
    run_gate
    [ "$status" -eq 0 ]
}

@test "consumes an artifact produced by another skill -> vocabulary PASS" {
    mkskill "$ROOT" producer "" "git-changes" alpha ""
    mkskill "$ROOT" alpha "git-changes" "" producer ""
    run_gate
    [ "$status" -eq 0 ]
}

@test "consumes a dangling token -> FAIL with consumes-vocabulary finding" {
    mkskill "$ROOT" alpha "totally-bogus" "" beta ""
    mkskill "$ROOT" beta
    run_gate
    [ "$status" -eq 1 ]
    [[ "$output" == *"consumes-vocabulary"* ]]
    [[ "$output" == *"totally-bogus"* ]]
    [[ "$output" == *"alpha/SKILL.md"* ]]
}

@test "metadata.dependencies pointing at non-skill -> FAIL" {
    mkskill "$ROOT" alpha "" "" "" "ghost"
    mkskill "$ROOT" beta "" "" alpha ""
    run_gate
    [ "$status" -eq 1 ]
    [[ "$output" == *"metadata-dependencies"* ]]
    [[ "$output" == *"ghost"* ]]
}

@test "un-allowlisted orphan -> FAIL" {
    mkskill "$ROOT" alpha "" "" beta ""
    mkskill "$ROOT" beta
    mkskill "$ROOT" lonely "repo-context" "result.json"   # zero skill edges
    run_gate
    [ "$status" -eq 1 ]
    [[ "$output" == *"[orphan] lonely/SKILL.md"* ]]
}

@test "allowlisted orphan -> PASS" {
    mkskill "$ROOT" alpha "" "" beta ""
    mkskill "$ROOT" beta
    mkskill "$ROOT" lonely "repo-context" "result.json"
    echo "lonely  # boundary leaf" > "$ALLOW"
    run_gate
    [ "$status" -eq 0 ]
}

@test "metadata.dependencies edge alone counts as connectivity" {
    # alpha has no consumes/context_rel; only metadata.dependencies -> beta.
    mkskill "$ROOT" alpha "" "" "" "beta"
    mkskill "$ROOT" beta
    run_gate
    [ "$status" -eq 0 ]
}

@test "--json emits machine-readable verdict with failures and orphans" {
    mkskill "$ROOT" alpha "totally-bogus" "" beta ""
    mkskill "$ROOT" beta
    run bash "$SCRIPT" --skills-root "$ROOT" --allowlist "$ALLOW" --json
    [ "$status" -eq 1 ]
    echo "$output" | jq empty
    [ "$(echo "$output" | jq -r '.verdict')" = "FAIL" ]
    [ "$(echo "$output" | jq -r '.failures[0].kind')" = "consumes-vocabulary" ]
    [ "$(echo "$output" | jq -r '.failures[0].skill')" = "alpha" ]
}

@test "consumes vs metadata.dependencies disagreement is reported, not fatal" {
    mkskill "$ROOT" alpha "beta" "" "" "gamma"
    mkskill "$ROOT" beta
    mkskill "$ROOT" gamma
    run_gate
    [ "$status" -eq 0 ]
    [[ "$output" == *"consumes(skills) != metadata.dependencies"* ]]
}
