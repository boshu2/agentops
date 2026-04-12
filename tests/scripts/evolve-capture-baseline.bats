#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/evolve-capture-baseline.sh"

    TMP_DIR="$(mktemp -d)"
    FAKE_REPO="$TMP_DIR/repo"
    MOCK_BIN="$TMP_DIR/bin"
    mkdir -p "$FAKE_REPO/scripts" "$MOCK_BIN"
    /bin/cp "$SCRIPT" "$FAKE_REPO/scripts/evolve-capture-baseline.sh"
    chmod +x "$FAKE_REPO/scripts/evolve-capture-baseline.sh"

    git init -q "$FAKE_REPO"
    git -C "$FAKE_REPO" config user.email "test@example.com"
    git -C "$FAKE_REPO" config user.name "Test User"
    echo "base" > "$FAKE_REPO/README.md"
    git -C "$FAKE_REPO" add README.md
    git -C "$FAKE_REPO" commit -q -m "base"
}

teardown() {
    rm -rf "$TMP_DIR"
}

write_mock_ao() {
    local body="$1"
    cat > "$MOCK_BIN/ao" <<EOF
#!/usr/bin/env bash
$body
EOF
    chmod +x "$MOCK_BIN/ao"
}

@test "evolve-capture-baseline.sh exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "evolve-capture-baseline.sh writes immutable baseline artifacts" {
    write_mock_ao 'cat <<'"'"'JSON'"'"'
{"goals":[{"id":"one","result":"pass"},{"id":"two","result":"fail"}]}
JSON'

    run env PATH="$MOCK_BIN:$PATH" bash "$FAKE_REPO/scripts/evolve-capture-baseline.sh" \
        --repo-root "$FAKE_REPO" \
        --label era-001 \
        --timeout 45

    [ "$status" -eq 0 ]
    baseline_file="$(find "$FAKE_REPO/.agents/evolve/fitness-baselines/era-001" -maxdepth 1 -type f -name '*.json' -print -quit)"
    [ -n "$baseline_file" ]
    [[ "$output" == ".agents/evolve/fitness-baselines/era-001/"*.json ]]
    [ ! -f "$FAKE_REPO/.agents/evolve/baselines/index.jsonl" ]
    [ ! -f "$FAKE_REPO/.agents/evolve/active-baseline.txt" ]
    [ ! -f "$FAKE_REPO/.agents/evolve/fitness-0-baseline.json" ]
}

@test "evolve-capture-baseline.sh can write legacy compatibility artifacts when requested" {
    write_mock_ao 'cat <<'"'"'JSON'"'"'
{"goals":[{"id":"one","result":"pass"},{"id":"two","result":"fail"}]}
JSON'

    run env PATH="$MOCK_BIN:$PATH" bash "$FAKE_REPO/scripts/evolve-capture-baseline.sh" \
        --repo-root "$FAKE_REPO" \
        --label era-compat \
        --legacy-path .agents/evolve/fitness-0-baseline.json \
        --active-path .agents/evolve/active-baseline.txt \
        --index-path .agents/evolve/fitness-baselines/index.jsonl
    [ "$status" -eq 0 ]

    run cat "$FAKE_REPO/.agents/evolve/active-baseline.txt"
    [ "$status" -eq 0 ]
    [[ "$output" == ".agents/evolve/fitness-baselines/era-compat/"*.json ]]
    [ -f "$FAKE_REPO/.agents/evolve/fitness-0-baseline.json" ]

    run jq -r '.label + ":" + (.goals_total|tostring)' "$FAKE_REPO/.agents/evolve/fitness-baselines/index.jsonl"
    [ "$status" -eq 0 ]
    [ "$output" = "era-compat:2" ]
}

@test "evolve-capture-baseline.sh refuses to overwrite labels without force" {
    write_mock_ao 'cat <<'"'"'JSON'"'"'
{"goals":[{"id":"one","result":"pass"}]}
JSON'

    run env PATH="$MOCK_BIN:$PATH" bash "$FAKE_REPO/scripts/evolve-capture-baseline.sh" \
        --repo-root "$FAKE_REPO" \
        --label era-dup
    [ "$status" -eq 0 ]

    run env PATH="$MOCK_BIN:$PATH" bash "$FAKE_REPO/scripts/evolve-capture-baseline.sh" \
        --repo-root "$FAKE_REPO" \
        --label era-dup

    [ "$status" -eq 1 ]
    [[ "$output" == *"baseline already exists"* ]]
}

@test "evolve-capture-baseline.sh fails on invalid JSON" {
    write_mock_ao 'echo "not-json"'

    run env PATH="$MOCK_BIN:$PATH" bash "$FAKE_REPO/scripts/evolve-capture-baseline.sh" \
        --repo-root "$FAKE_REPO" \
        --label era-bad

    [ "$status" -eq 1 ]
    [[ "$output" == *"not valid goals JSON"* ]]
    [ ! -d "$FAKE_REPO/.agents/evolve/fitness-baselines/era-bad" ] || [ -z "$(find "$FAKE_REPO/.agents/evolve/fitness-baselines/era-bad" -maxdepth 1 -type f -name '*.json' -print -quit)" ]
}
