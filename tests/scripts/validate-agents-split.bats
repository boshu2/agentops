#!/usr/bin/env bats
#
# tests/scripts/validate-agents-split.bats
# Regression coverage for scripts/validate-agents-split.sh after the root
# AGENTS-* sibling cutover: AGENTS.md must stay lean and point at the three
# detail owners under docs/.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/validate-agents-split.sh"

    TMP_DIR="$(mktemp -d)"
    WORK_REPO="$TMP_DIR/repo"

    mkdir -p "$WORK_REPO/scripts" \
        "$WORK_REPO/docs/contracts"
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
        echo "- [docs/agent-workflow-reference.md](docs/agent-workflow-reference.md)"
        echo "- [docs/CI-CD.md](docs/CI-CD.md)"
        echo "- [docs/contracts/codex-skill-api.md](docs/contracts/codex-skill-api.md)"
    } > "$WORK_REPO/AGENTS.md"

    printf '# workflow\n\nBack-link: [AGENTS.md](../AGENTS.md)\n' \
        > "$WORK_REPO/docs/agent-workflow-reference.md"
    printf '# ci\n\nBack-link: [AGENTS.md](../AGENTS.md)\n' \
        > "$WORK_REPO/docs/CI-CD.md"
    printf '# codex\n\nBack-link: [AGENTS.md](../../AGENTS.md)\n' \
        > "$WORK_REPO/docs/contracts/codex-skill-api.md"
}

@test "passes when AGENTS.md <=250 lines and three owners link bidirectionally" {
    write_valid_split

    run bash -c "cd '$WORK_REPO' && bash scripts/validate-agents-split.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
    [[ "$output" == *"15 checks"* ]]
}

@test "fails when AGENTS.md is missing" {
    write_valid_split
    rm "$WORK_REPO/AGENTS.md"

    run bash -c "cd '$WORK_REPO' && bash scripts/validate-agents-split.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"* ]] || [[ "$output" == *"does not exist"* ]]
}

@test "fails when AGENTS.md exceeds 250 lines" {
    write_valid_split
    for _ in $(seq 1 300); do
        echo "filler line" >> "$WORK_REPO/AGENTS.md"
    done

    run bash -c "cd '$WORK_REPO' && bash scripts/validate-agents-split.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"* ]]
    [[ "$output" == *"exceeds 250-line"* ]]
}

@test "fails when an owner file is missing" {
    write_valid_split
    rm "$WORK_REPO/docs/CI-CD.md"

    run bash -c "cd '$WORK_REPO' && bash scripts/validate-agents-split.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"* ]]
    [[ "$output" == *"missing owner"* ]]
    [[ "$output" == *"docs/CI-CD.md"* ]]
}

@test "fails when AGENTS.md does not link to an owner" {
    write_valid_split
    {
        echo "# Agent Instructions"
        echo "- [docs/agent-workflow-reference.md](docs/agent-workflow-reference.md)"
        echo "- [docs/CI-CD.md](docs/CI-CD.md)"
    } > "$WORK_REPO/AGENTS.md"

    run bash -c "cd '$WORK_REPO' && bash scripts/validate-agents-split.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *"does not link to docs/contracts/codex-skill-api.md"* ]]
}

@test "fails when an owner does not back-link to AGENTS.md" {
    write_valid_split
    printf '# ci\n\nNo back-link here.\n' > "$WORK_REPO/docs/CI-CD.md"

    run bash -c "cd '$WORK_REPO' && bash scripts/validate-agents-split.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *"does not back-link to AGENTS.md"* ]]
}

@test "fails when a legacy root sibling is still present" {
    write_valid_split
    printf '# leftover\n' > "$WORK_REPO/AGENTS-CI.md"

    run bash -c "cd '$WORK_REPO' && bash scripts/validate-agents-split.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *"legacy root sibling still present"* ]]
    [[ "$output" == *"AGENTS-CI.md"* ]]
}

@test "treats AGENTS.md at exactly the 250-line limit as passing" {
    write_valid_split
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
