#!/usr/bin/env bats
#
# tests/scripts/validate-agents-split.bats
# Regression coverage for scripts/validate-agents-split.sh (soc-vuu6.3).
#
# Sibling pattern: matches tests/scripts/validate-sovereignty-proof-citations.bats —
# script copied into a throwaway repo, fixtures synthesized per case, each
# case exercises one branch of the split-contract checks.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/validate-agents-split.sh"

    TMP_DIR="$(mktemp -d)"
    WORK_REPO="$TMP_DIR/repo"

    mkdir -p "$WORK_REPO/scripts"
    cp "$SCRIPT" "$WORK_REPO/scripts/validate-agents-split.sh"
    chmod +x "$WORK_REPO/scripts/validate-agents-split.sh"
}

teardown() {
    rm -rf "$TMP_DIR"
}

write_valid_split() {
    {
        echo "# Agent Instructions"
        echo ""
        echo "Links:"
        echo "- [AGENTS-WORKFLOW.md](AGENTS-WORKFLOW.md)"
        echo "- [AGENTS-CI.md](AGENTS-CI.md)"
        echo "- [AGENTS-CODEX.md](AGENTS-CODEX.md)"
        echo "- [AGENTS-RUNTIME.md](AGENTS-RUNTIME.md)"
    } > "$WORK_REPO/AGENTS.md"
    for sib in AGENTS-WORKFLOW.md AGENTS-CI.md AGENTS-CODEX.md AGENTS-RUNTIME.md; do
        printf '# %s\n\nBack-link: [AGENTS.md](AGENTS.md)\n' "$sib" > "$WORK_REPO/$sib"
    done
}

@test "passes when all 5 files exist with bidirectional links and AGENTS.md <=250 lines" {
    write_valid_split

    run bash -c "cd '$WORK_REPO' && bash scripts/validate-agents-split.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
    [[ "$output" == *"14 checks"* ]]
}

@test "fails when AGENTS.md is missing" {
    # Don't create AGENTS.md
    for sib in AGENTS-WORKFLOW.md AGENTS-CI.md AGENTS-CODEX.md AGENTS-RUNTIME.md; do
        printf '# %s\n\n[AGENTS.md](AGENTS.md)\n' "$sib" > "$WORK_REPO/$sib"
    done

    run bash -c "cd '$WORK_REPO' && bash scripts/validate-agents-split.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"* ]] || [[ "$output" == *"does not exist"* ]]
}

@test "fails when AGENTS.md exceeds 250 lines" {
    write_valid_split
    # Append 300 lines to AGENTS.md
    for _ in $(seq 1 300); do
        echo "filler line" >> "$WORK_REPO/AGENTS.md"
    done

    run bash -c "cd '$WORK_REPO' && bash scripts/validate-agents-split.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"* ]]
    [[ "$output" == *"exceeds 250-line"* ]]
}

@test "fails when a sibling file is missing" {
    write_valid_split
    rm "$WORK_REPO/AGENTS-CI.md"

    run bash -c "cd '$WORK_REPO' && bash scripts/validate-agents-split.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"* ]]
    [[ "$output" == *"missing sibling"* ]]
    [[ "$output" == *"AGENTS-CI.md"* ]]
}

@test "fails when AGENTS.md does not link to a sibling" {
    write_valid_split
    # Rewrite AGENTS.md without the AGENTS-CODEX.md link
    {
        echo "# Agent Instructions"
        echo "- [AGENTS-WORKFLOW.md](AGENTS-WORKFLOW.md)"
        echo "- [AGENTS-CI.md](AGENTS-CI.md)"
        echo "- [AGENTS-RUNTIME.md](AGENTS-RUNTIME.md)"
    } > "$WORK_REPO/AGENTS.md"

    run bash -c "cd '$WORK_REPO' && bash scripts/validate-agents-split.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *"does not link to AGENTS-CODEX.md"* ]]
}

@test "fails when a sibling does not back-link to AGENTS.md" {
    write_valid_split
    # Rewrite a sibling without the back-link
    printf '# AGENTS-CI.md\n\nNo back-link here.\n' > "$WORK_REPO/AGENTS-CI.md"

    run bash -c "cd '$WORK_REPO' && bash scripts/validate-agents-split.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *"does not back-link to AGENTS.md"* ]]
}

@test "treats AGENTS.md at exactly the 250-line limit as passing" {
    write_valid_split
    # Pad to exactly 250 lines
    current=$(wc -l < "$WORK_REPO/AGENTS.md")
    needed=$((250 - current))
    for _ in $(seq 1 "$needed"); do
        echo "filler" >> "$WORK_REPO/AGENTS.md"
    done
    actual=$(wc -l < "$WORK_REPO/AGENTS.md")
    [ "$actual" -eq 250 ]

    run bash -c "cd '$WORK_REPO' && bash scripts/validate-agents-split.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}
