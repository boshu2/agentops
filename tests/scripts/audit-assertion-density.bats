#!/usr/bin/env bats

# Regression test for audit-assertion-density.sh's --scope flag.
# Pre-cycle: the script hardcoded `*coverage*_test.go` as the find pattern,
# silently auditing zero files because cov*_test.go is banned by CLAUDE.md.
# This test pins the new default to all *_test.go files and verifies the
# legacy "coverage" scope alias still works.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/audit-assertion-density.sh"

    TMP_DIR="$(mktemp -d)"
    SAMPLES="$TMP_DIR/pkg"
    mkdir -p "$SAMPLES"

    cat > "$SAMPLES/dense_test.go" <<'EOF'
package pkg
import "testing"
func TestA(t *testing.T) {
    if 1 != 1 { t.Errorf("never") }
    if 2 != 2 { t.Fatal("never") }
}
EOF
    cat > "$SAMPLES/hollow_test.go" <<'EOF'
package pkg
import "testing"
func TestB(t *testing.T) { _ = 1 }
func TestC(t *testing.T) { _ = 2 }
func TestD(t *testing.T) { _ = 3 }
EOF
    cat > "$SAMPLES/coverage_test.go" <<'EOF'
package pkg
import "testing"
func TestCov(t *testing.T) {
    if 1 != 1 { t.Errorf("never") }
    if 2 != 2 { t.Fatal("never") }
}
EOF
}

teardown() {
    rm -rf "$TMP_DIR"
}

@test "script exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "default scope audits ALL *_test.go files" {
    run bash "$SCRIPT" "$SAMPLES"
    [ "$status" -eq 0 ]
    [[ "$output" == *"dense_test.go"* ]]
    [[ "$output" == *"hollow_test.go"* ]]
    [[ "$output" == *"coverage_test.go"* ]]
    [[ "$output" == *"3 test files"* ]]
}

@test "default scope flags hollow_test.go as below threshold" {
    run bash "$SCRIPT" "$SAMPLES"
    [ "$status" -eq 0 ]
    [[ "$output" == *"HOLLOW: $SAMPLES/hollow_test.go"* ]]
}

@test "--scope coverage restricts to legacy *coverage*_test.go" {
    run bash "$SCRIPT" --scope coverage "$SAMPLES"
    [ "$status" -eq 0 ]
    [[ "$output" == *"coverage_test.go"* ]]
    [[ "$output" != *"hollow_test.go"* ]]
    [[ "$output" != *"dense_test.go"* ]]
    [[ "$output" == *"1 test files"* ]]
}

@test "--scope all is an explicit alias for the new default" {
    run bash "$SCRIPT" --scope all "$SAMPLES"
    [ "$status" -eq 0 ]
    [[ "$output" == *"3 test files"* ]]
}

@test "--check exits 1 when hollow tests are present" {
    run bash "$SCRIPT" --check "$SAMPLES"
    [ "$status" -eq 1 ]
    [[ "$output" == *"FAIL"* ]]
}

@test "--check exits 0 when no hollow tests in scope" {
    run bash "$SCRIPT" --check --scope coverage "$SAMPLES"
    [ "$status" -eq 0 ]
}

@test "custom glob via --scope is honored" {
    run bash "$SCRIPT" --scope 'hollow_test.go' "$SAMPLES"
    [ "$status" -eq 0 ]
    [[ "$output" == *"hollow_test.go"* ]]
    [[ "$output" != *"dense_test.go"* ]]
}
