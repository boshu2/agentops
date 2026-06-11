#!/usr/bin/env bats
# Tests for scripts/validate-bd-closeout-contract.sh — the INVERTED (br-era)
# closeout doc-contract gate (pre-push check 19b). The gate asserts
# AGENTS-WORKFLOW.md documents the br flush discipline and carries no live
# bd/Dolt closeout instructions. CLOSEOUT_CONTRACT_WORKFLOW_DOC points the
# gate at fixture docs so the real ones are never modified.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/validate-bd-closeout-contract.sh"
    TMP_DIR="$(mktemp -d)"
}

teardown() {
    rm -rf "$TMP_DIR"
}

write_doc() {
    local path="$TMP_DIR/AGENTS-WORKFLOW.md"
    printf '%s\n' "$@" > "$path"
    echo "$path"
}

@test "passes against the repo's live flipped AGENTS-WORKFLOW.md" {
    run "$SCRIPT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"BR_CLOSEOUT_CONTRACT: PASS"* ]]
}

@test "passes when doc has br flush discipline and no bd dolt instructions" {
    doc="$(write_doc \
        'Closeout: br sync --flush-only   # export DB -> _beads JSONL' \
        'br sync --flush-only && git -C _beads add -A && git -C _beads push  # if tracker changed')"
    CLOSEOUT_CONTRACT_WORKFLOW_DOC="$doc" run "$SCRIPT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"BR_CLOSEOUT_CONTRACT: PASS"* ]]
}

@test "fails when doc still carries bd dolt push closeout instructions" {
    doc="$(write_doc \
        'br sync --flush-only && git -C _beads add -A' \
        'bd dolt push  # only if a real Dolt remote is configured')"
    CLOSEOUT_CONTRACT_WORKFLOW_DOC="$doc" run "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"still contains live bd/Dolt closeout instructions"* ]]
    [[ "$output" == *"bd dolt push"* ]]
}

@test "fails when doc still carries bd dolt commit closeout instructions" {
    doc="$(write_doc \
        'br sync --flush-only && git -C _beads add -A' \
        'then bd dolt commit before push')"
    CLOSEOUT_CONTRACT_WORKFLOW_DOC="$doc" run "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"still contains live bd/Dolt closeout instructions"* ]]
}

@test "fails when br flush discipline is missing" {
    doc="$(write_doc 'git -C _beads add -A')"
    CLOSEOUT_CONTRACT_WORKFLOW_DOC="$doc" run "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"must document the br flush discipline (br sync --flush-only)"* ]]
}

@test "fails when ledger staging instruction is missing" {
    doc="$(write_doc 'br sync --flush-only')"
    CLOSEOUT_CONTRACT_WORKFLOW_DOC="$doc" run "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"must document the private-ledger sync (git -C _beads add/commit/push)"* ]]
}

@test "fails when the workflow doc is missing" {
    CLOSEOUT_CONTRACT_WORKFLOW_DOC="$TMP_DIR/does-not-exist.md" run "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"missing required file"* ]]
}

@test "fails when doc stages _beads into the public repo" {
    doc="$(write_doc 'br sync --flush-only && git -C _beads add -A' 'git add _beads/*.jsonl')"
    CLOSEOUT_CONTRACT_WORKFLOW_DOC="$doc" run "$SCRIPT"
    [ "$status" -eq 1 ]
    [[ "$output" == *"stages _beads/ into the PUBLIC repo"* ]]
}
