#!/usr/bin/env bats
# Tests for scripts/check-no-apparatus-regrowth.sh — the anti-regeneration
# stay-removed gate. Drives the script against a temp repo + temp manifest so
# the result never depends on the real repo's current tree state.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-no-apparatus-regrowth.sh"
    TMP_DIR="$(mktemp -d)"
    FAKE_ROOT="$TMP_DIR/repo"
    MANIFEST="$TMP_DIR/removed-apparatus.txt"
    mkdir -p "$FAKE_ROOT"
    cat > "$MANIFEST" <<'EOF'
# comment line — ignored
cli/internal/wikiworker    # removed, PR #589
cli/internal/plans
cli/internal/worker
EOF
}

teardown() {
    rm -rf "$TMP_DIR"
}

@test "PASS when every removed surface is absent" {
    run bash "$SCRIPT" --root "$FAKE_ROOT" --manifest "$MANIFEST"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
    [[ "$output" == *"3 teardown-removed surface(s) stay removed"* ]]
}

@test "FAIL with exit 1 when a removed surface regrows" {
    mkdir -p "$FAKE_ROOT/cli/internal/plans"

    run bash "$SCRIPT" --root "$FAKE_ROOT" --manifest "$MANIFEST"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"* ]]
    [[ "$output" == *"cli/internal/plans"* ]]
}

@test "FAIL names every regrown surface, not just the first" {
    mkdir -p "$FAKE_ROOT/cli/internal/plans" "$FAKE_ROOT/cli/internal/worker"

    run bash "$SCRIPT" --root "$FAKE_ROOT" --manifest "$MANIFEST"
    [ "$status" -eq 1 ]
    [[ "$output" == *"2 teardown-removed surface(s) regrew"* ]]
    [[ "$output" == *"cli/internal/plans"* ]]
    [[ "$output" == *"cli/internal/worker"* ]]
}

@test "regrown file (not just dir) also fails" {
    # A removed surface can be a single file (e.g. cli/internal/bridge/gc.go).
    echo "cli/internal/bridge/gc.go" > "$MANIFEST"
    mkdir -p "$FAKE_ROOT/cli/internal/bridge"
    echo "package bridge" > "$FAKE_ROOT/cli/internal/bridge/gc.go"

    run bash "$SCRIPT" --root "$FAKE_ROOT" --manifest "$MANIFEST"
    [ "$status" -eq 1 ]
    [[ "$output" == *"cli/internal/bridge/gc.go"* ]]
}

@test "--json emits machine-readable pass result" {
    run bash "$SCRIPT" --json --root "$FAKE_ROOT" --manifest "$MANIFEST"
    [ "$status" -eq 0 ]
    [[ "$output" == *'"status":"pass"'* ]]
    [[ "$output" == *'"checked":3'* ]]
    [[ "$output" == *'"regrown":[]'* ]]
}

@test "--json emits machine-readable fail result with regrown path" {
    mkdir -p "$FAKE_ROOT/cli/internal/worker"

    run bash "$SCRIPT" --json --root "$FAKE_ROOT" --manifest "$MANIFEST"
    [ "$status" -eq 1 ]
    [[ "$output" == *'"status":"fail"'* ]]
    [[ "$output" == *'"cli/internal/worker"'* ]]
}

@test "missing manifest fails with exit 1" {
    run bash "$SCRIPT" --root "$FAKE_ROOT" --manifest "$TMP_DIR/does-not-exist.txt"
    [ "$status" -eq 1 ]
    [[ "$output" == *"manifest not found"* ]]
}

@test "the real committed manifest passes on the real repo tree" {
    # Guards against shipping a manifest that lists a still-present surface.
    run bash "$SCRIPT"
    [ "$status" -eq 0 ]
    [[ "$output" == *"PASS"* ]]
}
