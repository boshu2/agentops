#!/usr/bin/env bats
# tests/scripts/test-check-test-fixture-parity.bats
#
# Coverage for scripts/check-test-fixture-parity.sh.
# Cases:
#   1. PASS on a fixture mirroring the current main repo (clean parity).
#   2. FAIL: hook present, no test reference (missing coverage).
#   3. FAIL: helper referenced from pre-push-gate.sh, no make_stub in bats.
#   4. Replay regression: synthesized 5da522e9-style break (W3 amendment).
#   5. Bypass: AGENTOPS_PARITY_GATE_DISABLED=1 -> exit 0.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$REPO_ROOT/scripts/check-test-fixture-parity.sh"
    TMP_DIR="$(mktemp -d)"
    FAKE_REPO="$TMP_DIR/repo"
    mkdir -p "$FAKE_REPO/hooks" \
             "$FAKE_REPO/scripts" \
             "$FAKE_REPO/tests/hooks" \
             "$FAKE_REPO/tests/scripts" \
             "$FAKE_REPO/tests/skills"
}

teardown() {
    rm -rf "$TMP_DIR"
}

# Helper: write a minimal pre-push-gate.sh referencing $1, $2, ... .sh helpers.
write_gate() {
    local out="$FAKE_REPO/scripts/pre-push-gate.sh"
    {
        echo '#!/usr/bin/env bash'
        echo 'set -euo pipefail'
        for ref in "$@"; do
            echo "# bash $ref"
        done
    } > "$out"
    chmod +x "$out"
}

# Helper: write a minimal pre-push-gate.bats with make_stub for $1, $2, ...
write_bats_with_stubs() {
    local out="$FAKE_REPO/tests/scripts/pre-push-gate.bats"
    {
        echo '#!/usr/bin/env bats'
        echo 'setup() {'
        for ref in "$@"; do
            echo "    make_stub \"\$FAKE_REPO/$ref\""
        done
        echo '}'
    } > "$out"
}

# Helper: write tests/hooks/test-hooks.{sh,bats} that reference $1, $2, ...
write_hook_coverage() {
    {
        echo '#!/usr/bin/env bash'
        for ref in "$@"; do
            echo "# covers: $ref"
        done
    } > "$FAKE_REPO/tests/hooks/test-hooks.sh"
    {
        echo '#!/usr/bin/env bats'
        for ref in "$@"; do
            echo "# covers: $ref"
        done
    } > "$FAKE_REPO/tests/hooks/test-hooks.bats"
}

@test "PASS on clean fixture (all hooks covered, all helpers stubbed)" {
    echo '#!/usr/bin/env bash' > "$FAKE_REPO/hooks/foo.sh"
    echo '#!/usr/bin/env bash' > "$FAKE_REPO/hooks/bar.sh"
    write_hook_coverage "foo" "bar"
    write_gate "scripts/helper-a.sh" "scripts/helper-b.sh"
    write_bats_with_stubs "scripts/helper-a.sh" "scripts/helper-b.sh"

    run "$SCRIPT" "$FAKE_REPO"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "FAIL: hook present without test coverage" {
    echo '#!/usr/bin/env bash' > "$FAKE_REPO/hooks/foo.sh"
    echo '#!/usr/bin/env bash' > "$FAKE_REPO/hooks/orphan.sh"
    write_hook_coverage "foo"   # 'orphan' deliberately missing
    write_gate
    write_bats_with_stubs

    run "$SCRIPT" "$FAKE_REPO"
    [ "$status" -eq 1 ]
    [[ "$output" == *"hooks with no test coverage"* ]]
    [[ "$output" == *"orphan"* ]]
}

@test "FAIL: helper referenced from pre-push-gate.sh without bats make_stub" {
    write_hook_coverage
    write_gate "scripts/orphan-helper.sh"
    write_bats_with_stubs   # no stub for orphan-helper

    run "$SCRIPT" "$FAKE_REPO"
    [ "$status" -eq 1 ]
    [[ "$output" == *"pre-push helpers without bats stub"* ]]
    [[ "$output" == *"scripts/orphan-helper.sh"* ]]
}

@test "Replay regression: 5da522e9-style parity break (W3 amendment)" {
    # Synthesize the deltas from commit 5da522e9: new hooks/edit-audit.sh and
    # scripts/check-retrieval-manifest-paths.sh referenced from pre-push-gate.sh,
    # with no corresponding test/bats updates.
    echo '#!/usr/bin/env bash' > "$FAKE_REPO/hooks/edit-audit.sh"
    echo '#!/usr/bin/env bash' > "$FAKE_REPO/hooks/eval-verdict-compiler.sh"
    write_hook_coverage  # both hooks deliberately uncovered
    write_gate "scripts/check-retrieval-manifest-paths.sh"
    write_bats_with_stubs  # no stub deliberately

    run "$SCRIPT" "$FAKE_REPO"
    [ "$status" -eq 1 ]
    [[ "$output" == *"hooks with no test coverage"* ]]
    [[ "$output" == *"edit-audit"* ]]
    [[ "$output" == *"eval-verdict-compiler"* ]]
    [[ "$output" == *"pre-push helpers without bats stub"* ]]
    [[ "$output" == *"check-retrieval-manifest-paths.sh"* ]]
}

@test "Bypass via AGENTOPS_PARITY_GATE_DISABLED" {
    echo '#!/usr/bin/env bash' > "$FAKE_REPO/hooks/orphan.sh"
    write_hook_coverage   # no coverage -> would normally FAIL
    write_gate
    write_bats_with_stubs

    AGENTOPS_PARITY_GATE_DISABLED=1 run "$SCRIPT" "$FAKE_REPO"
    [ "$status" -eq 0 ]
    [[ "$output" == *"skipped"* ]]
}

@test "Self-reference exclusion: pre-push-gate.sh referencing itself does not require a self-stub" {
    write_hook_coverage
    # pre-push-gate.sh references itself (e.g., 'echo scripts/pre-push-gate.sh')
    {
        echo '#!/usr/bin/env bash'
        echo "# scripts/pre-push-gate.sh"
    } > "$FAKE_REPO/scripts/pre-push-gate.sh"
    chmod +x "$FAKE_REPO/scripts/pre-push-gate.sh"
    write_bats_with_stubs

    run "$SCRIPT" "$FAKE_REPO"
    [ "$status" -eq 0 ]
}
