#!/usr/bin/env bats
#
# tests/scripts/validate-sovereignty-proof-citations.bats
# Regression coverage for scripts/validate-sovereignty-proof-citations.sh.
#
# Sibling pattern: matches tests/scripts/check-removed-symbol-refs.bats —
# script copied into a throwaway repo with synthesized docs/sovereignty-proof/
# fixtures, each case exercises one branch of the validator.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/validate-sovereignty-proof-citations.sh"

    TMP_DIR="$(mktemp -d)"
    WORK_REPO="$TMP_DIR/repo"

    mkdir -p "$WORK_REPO/scripts" "$WORK_REPO/docs/sovereignty-proof" "$WORK_REPO/cli/cmd/ao"
    cp "$SCRIPT" "$WORK_REPO/scripts/validate-sovereignty-proof-citations.sh"
    chmod +x "$WORK_REPO/scripts/validate-sovereignty-proof-citations.sh"

    # Synthesize a 50-line file we can cite into
    : > "$WORK_REPO/cli/cmd/ao/sample.go"
    for i in $(seq 1 50); do
        printf 'line %s\n' "$i" >> "$WORK_REPO/cli/cmd/ao/sample.go"
    done
}

teardown() {
    rm -rf "$TMP_DIR"
}

@test "passes when every cited file:line resolves" {
    cat > "$WORK_REPO/docs/sovereignty-proof/index.md" <<'EOF'
# Page
See cli/cmd/ao/sample.go:1 and cli/cmd/ao/sample.go:10-25 for evidence.
EOF

    run bash -c "cd '$WORK_REPO' && bash scripts/validate-sovereignty-proof-citations.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
    [[ "$output" == *"2 citations"* ]]
}

@test "fails when cited file does not exist" {
    cat > "$WORK_REPO/docs/sovereignty-proof/index.md" <<'EOF'
# Page
Bad citation: cli/cmd/ao/ghost.go:5
EOF

    run bash -c "cd '$WORK_REPO' && bash scripts/validate-sovereignty-proof-citations.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"* ]]
    [[ "$output" == *"ghost.go"* ]]
    [[ "$output" == *"file does not exist"* ]]
}

@test "fails when cited line exceeds file length" {
    cat > "$WORK_REPO/docs/sovereignty-proof/index.md" <<'EOF'
# Page
Drifted: cli/cmd/ao/sample.go:9999
EOF

    run bash -c "cd '$WORK_REPO' && bash scripts/validate-sovereignty-proof-citations.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"* ]]
    [[ "$output" == *"exceeds file's"* ]]
}

@test "ignores citations inside fenced code blocks" {
    {
        printf '# Page\n'
        printf 'Real cite: cli/cmd/ao/sample.go:1\n\n'
        printf '%s\n' '```'
        printf 'This is not a citation: cli/cmd/ao/ghost.go:99\n'
        printf '%s\n' '```'
    } > "$WORK_REPO/docs/sovereignty-proof/index.md"

    run bash -c "cd '$WORK_REPO' && bash scripts/validate-sovereignty-proof-citations.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
    [[ "$output" == *"1 citations"* ]]
}

@test "recognizes citations inside backticks" {
    {
        printf '# Page\n'
        printf 'See `cli/cmd/ao/sample.go:5` for the cite-in-backticks shape.\n'
    } > "$WORK_REPO/docs/sovereignty-proof/index.md"

    run bash -c "cd '$WORK_REPO' && bash scripts/validate-sovereignty-proof-citations.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
    [[ "$output" == *"1 citations"* ]]
}

@test "scans multiple pages and aggregates" {
    cat > "$WORK_REPO/docs/sovereignty-proof/index.md" <<'EOF'
# Index
cli/cmd/ao/sample.go:1
EOF
    mkdir -p "$WORK_REPO/docs/sovereignty-proof/evidence"
    cat > "$WORK_REPO/docs/sovereignty-proof/evidence/case-a.md" <<'EOF'
cli/cmd/ao/sample.go:20
EOF
    cat > "$WORK_REPO/docs/sovereignty-proof/evidence/case-b.md" <<'EOF'
cli/cmd/ao/sample.go:40
EOF

    run bash -c "cd '$WORK_REPO' && bash scripts/validate-sovereignty-proof-citations.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *"scanned 3 pages"* ]]
    [[ "$output" == *"3 citations"* ]]
}

@test "exits 2 when sovereignty-proof dir is missing" {
    rm -rf "$WORK_REPO/docs/sovereignty-proof"

    run bash -c "cd '$WORK_REPO' && bash scripts/validate-sovereignty-proof-citations.sh"
    [ "$status" -eq 2 ]
    [[ "$output" == *"does not exist"* ]]
}

@test "exits 2 when sovereignty-proof dir contains no markdown" {
    # Dir exists from setup, but empty
    : > "$WORK_REPO/docs/sovereignty-proof/.keep"

    run bash -c "cd '$WORK_REPO' && bash scripts/validate-sovereignty-proof-citations.sh"
    [ "$status" -eq 2 ]
    [[ "$output" == *"no .md files"* ]]
}
