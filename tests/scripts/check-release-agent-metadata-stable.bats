#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-release-agent-metadata-stable.sh"
    TMP_REPO="$(mktemp -d)"

    git -C "$TMP_REPO" init -q
    mkdir -p "$TMP_REPO/.agents/findings" "$TMP_REPO/.agents/ao"
    cat > "$TMP_REPO/.agents/findings/f-release.md" <<'EOF'
---
id: f-release
hit_count: 1
---
# Finding
EOF
    printf '{"artifact_path":".agents/findings/f-release.md"}\n' > "$TMP_REPO/.agents/ao/citations.jsonl"
    git -C "$TMP_REPO" add .agents/findings/f-release.md .agents/ao/citations.jsonl
}

teardown() {
    rm -rf "$TMP_REPO"
}

@test "passes when guarded command leaves tracked metadata unchanged" {
    run "$SCRIPT" --repo-root "$TMP_REPO" -- bash -c 'test -f "$1/.agents/findings/f-release.md"' _ "$TMP_REPO"
    [ "$status" -eq 0 ]
}

@test "fails when guarded command mutates tracked finding metadata" {
    run "$SCRIPT" --repo-root "$TMP_REPO" -- bash -c 'printf "\nchanged\n" >> "$1/.agents/findings/f-release.md"' _ "$TMP_REPO"
    [ "$status" -eq 1 ]
    [[ "$output" == *"mutated tracked AgentOps finding/citation metadata"* ]]
}

@test "opt-in environment allows tracked metadata mutation" {
    run env AGENTOPS_RELEASE_ALLOW_AGENT_MUTATIONS=1 "$SCRIPT" --repo-root "$TMP_REPO" -- bash -c 'printf "\nchanged\n" >> "$1/.agents/findings/f-release.md"' _ "$TMP_REPO"
    [ "$status" -eq 0 ]
    [[ "$output" == *"mutation guard disabled"* ]]
}
