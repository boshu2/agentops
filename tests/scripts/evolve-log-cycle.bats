#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/evolve-log-cycle.sh"

    TMP_DIR="$(mktemp -d)"
    FAKE_REPO="$TMP_DIR/repo"
    mkdir -p "$FAKE_REPO/scripts" "$FAKE_REPO/.agents/evolve"
    /bin/cp "$SCRIPT" "$FAKE_REPO/scripts/evolve-log-cycle.sh"
    chmod +x "$FAKE_REPO/scripts/evolve-log-cycle.sh"

    git init -q "$FAKE_REPO"
    git -C "$FAKE_REPO" config user.email "test@example.com"
    git -C "$FAKE_REPO" config user.name "Test User"

    cat > "$FAKE_REPO/README.md" <<'EOF'
base
EOF
    git -C "$FAKE_REPO" add README.md
    git -C "$FAKE_REPO" commit -q -m "base"
    BASE_SHA="$(git -C "$FAKE_REPO" rev-parse --short HEAD)"
}

teardown() {
    rm -rf "$TMP_DIR"
}

@test "evolve-log-cycle.sh exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "evolve-log-cycle.sh writes canonical productive rows with distinct log_sha" {
    echo "real change" >> "$FAKE_REPO/README.md"
    git -C "$FAKE_REPO" add README.md
    git -C "$FAKE_REPO" commit -q -m "real change"
    CANONICAL_SHA="$(git -C "$FAKE_REPO" rev-parse --short HEAD)"
    git -C "$FAKE_REPO" commit --allow-empty -q -m "log commit"
    LOG_SHA="$(git -C "$FAKE_REPO" rev-parse --short HEAD)"

    run bash "$FAKE_REPO/scripts/evolve-log-cycle.sh" \
        --repo-root "$FAKE_REPO" \
        --cycle 1 \
        --target cmd-ao-coverage-floor \
        --result improved \
        --canonical-sha "$CANONICAL_SHA" \
        --log-sha "$LOG_SHA" \
        --cycle-start-sha "$BASE_SHA" \
        --goals-passing 17 \
        --goals-total 18 \
        --quality-score 90 \
        --timestamp 2026-03-07T16:00:00Z

    [ "$status" -eq 0 ]
    [[ "$output" == *'"canonical_sha":"'"$CANONICAL_SHA"'"'* ]]
    [[ "$output" == *'"log_sha":"'"$LOG_SHA"'"'* ]]
    run jq -r '.result' "$FAKE_REPO/.agents/evolve/cycle-history.jsonl"
    [ "$status" -eq 0 ]
    [ "$output" = "improved" ]
}

@test "evolve-log-cycle.sh downgrades artifact-only improved cycles to unchanged" {
    mkdir -p "$FAKE_REPO/.agents/work"
    echo "artifact only" > "$FAKE_REPO/.agents/work/note.txt"
    git -C "$FAKE_REPO" add .agents/work/note.txt
    git -C "$FAKE_REPO" commit -q -m "artifact only"
    AGENT_SHA="$(git -C "$FAKE_REPO" rev-parse --short HEAD)"

    run bash "$FAKE_REPO/scripts/evolve-log-cycle.sh" \
        --repo-root "$FAKE_REPO" \
        --cycle 1 \
        --target flywheel-proof \
        --result improved \
        --canonical-sha "$AGENT_SHA" \
        --cycle-start-sha "$BASE_SHA" \
        --goals-passing 16 \
        --goals-total 18

    [ "$status" -eq 0 ]
    [[ "$output" == *"downgraded cycle 1 to unchanged"* || "$output" == *'"result":"unchanged"'* ]]
    run jq -r '.result' "$FAKE_REPO/.agents/evolve/cycle-history.jsonl"
    [ "$status" -eq 0 ]
    [ "$output" = "unchanged" ]
    run jq -r 'has("canonical_sha")' "$FAKE_REPO/.agents/evolve/cycle-history.jsonl"
    [ "$status" -eq 0 ]
    [ "$output" = "false" ]
}

@test "evolve-log-cycle.sh requires idle target for direct unchanged cycles" {
    run bash "$FAKE_REPO/scripts/evolve-log-cycle.sh" \
        --repo-root "$FAKE_REPO" \
        --cycle 1 \
        --target flywheel-proof \
        --result unchanged

    [ "$status" -eq 1 ]
    [[ "$output" == *"unchanged results must use --target idle"* ]]

    run bash "$FAKE_REPO/scripts/evolve-log-cycle.sh" \
        --repo-root "$FAKE_REPO" \
        --cycle 1 \
        --target idle \
        --result unchanged \
        --timestamp 2026-03-07T16:00:00Z

    [ "$status" -eq 0 ]
    run jq -r '.target' "$FAKE_REPO/.agents/evolve/cycle-history.jsonl"
    [ "$status" -eq 0 ]
    [ "$output" = "idle" ]
}

@test "evolve-log-cycle.sh rejects non-numeric productive goal counts" {
    echo "real change" >> "$FAKE_REPO/README.md"
    git -C "$FAKE_REPO" add README.md
    git -C "$FAKE_REPO" commit -q -m "real change"
    CANONICAL_SHA="$(git -C "$FAKE_REPO" rev-parse --short HEAD)"

    run bash "$FAKE_REPO/scripts/evolve-log-cycle.sh" \
        --repo-root "$FAKE_REPO" \
        --cycle 1 \
        --target cmd-ao-coverage-floor \
        --result improved \
        --canonical-sha "$CANONICAL_SHA" \
        --cycle-start-sha "$BASE_SHA" \
        --goals-passing null \
        --goals-total 18

    [ "$status" -eq 1 ]
    [[ "$output" == *"--goals-passing must be numeric"* ]]
}

@test "evolve-log-cycle.sh rejects non-sequential cycle numbers" {
    echo "real change" >> "$FAKE_REPO/README.md"
    git -C "$FAKE_REPO" add README.md
    git -C "$FAKE_REPO" commit -q -m "real change"
    CANONICAL_SHA="$(git -C "$FAKE_REPO" rev-parse --short HEAD)"

    run bash "$FAKE_REPO/scripts/evolve-log-cycle.sh" \
        --repo-root "$FAKE_REPO" \
        --cycle 1 \
        --target cmd-ao-coverage-floor \
        --result improved \
        --canonical-sha "$CANONICAL_SHA" \
        --cycle-start-sha "$BASE_SHA" \
        --goals-passing 17 \
        --goals-total 18
    [ "$status" -eq 0 ]

    run bash "$FAKE_REPO/scripts/evolve-log-cycle.sh" \
        --repo-root "$FAKE_REPO" \
        --cycle 3 \
        --target cmd-ao-coverage-floor \
        --result regressed \
        --canonical-sha "$CANONICAL_SHA" \
        --goals-passing 16 \
        --goals-total 18

    [ "$status" -eq 1 ]
    [[ "$output" == *"expected cycle 2"* ]]
}
