#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

PASS=0
FAIL=0

pass() { echo "PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL: $1"; FAIL=$((FAIL + 1)); }

check_contains() {
    local file="$1"
    local pattern="$2"
    local message="$3"
    if grep -qE "$pattern" "$file"; then
        pass "$message"
    else
        fail "$message"
    fi
}

EVOLVE="$REPO_ROOT/skills/evolve/SKILL.md"
EVOLVE_EXAMPLES="$REPO_ROOT/skills/evolve/references/examples.md"
RPI="$REPO_ROOT/skills/rpi/SKILL.md"
RPI_CONTRACT="$REPO_ROOT/skills/rpi/references/phase-data-contracts.md"
INDEX="$REPO_ROOT/docs/documentation-index.md"
PROFILE_DOC="$REPO_ROOT/docs/contracts/repo-execution-profile.md"
PROFILE_SCHEMA="$REPO_ROOT/docs/contracts/repo-execution-profile.schema.json"
PROGRAM_DOC="$REPO_ROOT/docs/contracts/autodev-program.md"
CODEX_EVOLVE_PROMPT="$REPO_ROOT/skills-codex/evolve/prompt.md"

check_contains "$EVOLVE" 'repo execution profile' "/evolve documents repo execution profile bootstrap"
check_contains "$EVOLVE" 'startup_reads' "/evolve documents ordered startup reads"
check_contains "$EVOLVE" 'validation_commands' "/evolve records repo validation commands"
check_contains "$EVOLVE" 'definition_of_done' "/evolve records repo done criteria"
check_contains "$EVOLVE_EXAMPLES" 'repo execution profile' "/evolve examples mention repo bootstrap"
check_contains "$EVOLVE" 'PROGRAM.md' "/evolve documents PROGRAM.md execution layer"
check_contains "$EVOLVE" 'AUTODEV.md' "/evolve documents AUTODEV fallback"
check_contains "$EVOLVE" 'mutable_scope' "/evolve caches mutable scope"
check_contains "$EVOLVE" 'immutable_scope' "/evolve caches immutable scope"
check_contains "$EVOLVE" 'decision_policy' "/evolve caches decision policy"
check_contains "$EVOLVE" 'stop_conditions' "/evolve caches stop conditions"

check_contains "$RPI" 'execution packet' "/rpi documents execution packet handoff"
check_contains "$RPI" 'contract_surfaces' "/rpi documents execution packet contract surfaces"
check_contains "$RPI" 'done_criteria' "/rpi documents execution packet done criteria"
check_contains "$RPI_CONTRACT" 'repo execution profile' "/rpi phase data contract names the repo profile"
check_contains "$RPI_CONTRACT" 'execution_packet' "/rpi phase data contract names the execution packet artifact"
check_contains "$CODEX_EVOLVE_PROMPT" 'PROGRAM.md' "codex evolve prompt mentions PROGRAM.md execution layer"
check_contains "$CODEX_EVOLVE_PROMPT" 'mutable scope' "codex evolve prompt constrains mutable scope"
check_contains "$CODEX_EVOLVE_PROMPT" 'decision policy' "codex evolve prompt mentions decision policy gate"

check_contains "$INDEX" 'contracts/repo-execution-profile.md' "docs index catalogs repo execution profile doc"
check_contains "$INDEX" 'contracts/repo-execution-profile.schema.json' "docs index catalogs repo execution profile schema"
check_contains "$INDEX" 'contracts/autodev-program.md' "docs index catalogs autodev program doc"
check_contains "$PROFILE_DOC" 'repo-execution-profile.schema.json' "repo execution profile doc references its schema"
check_contains "$PROFILE_SCHEMA" '"definition_of_done"' "repo execution profile schema includes definition_of_done"
check_contains "$PROGRAM_DOC" 'PROGRAM.md' "autodev program doc names PROGRAM.md"
check_contains "$PROGRAM_DOC" 'AUTODEV.md' "autodev program doc documents AUTODEV fallback"
check_contains "$PROGRAM_DOC" 'Mutable Scope' "autodev program doc defines mutable scope"
check_contains "$PROGRAM_DOC" 'Decision Policy' "autodev program doc defines decision policy"

echo
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
