#!/usr/bin/env bats
# tests/scripts/test-check-test-fixture-parity.bats
#
# Coverage for scripts/check-test-fixture-parity.sh (hooks-only after Wave 2).

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

@test "PASS on clean fixture (all hooks covered)" {
    echo '#!/usr/bin/env bash' > "$FAKE_REPO/hooks/foo.sh"
    echo '#!/usr/bin/env bash' > "$FAKE_REPO/hooks/bar.sh"
    write_hook_coverage "foo" "bar"

    run "$SCRIPT" "$FAKE_REPO"
    [ "$status" -eq 0 ]
    [ -z "$output" ]
}

@test "FAIL: hook present without test coverage" {
    echo '#!/usr/bin/env bash' > "$FAKE_REPO/hooks/foo.sh"
    echo '#!/usr/bin/env bash' > "$FAKE_REPO/hooks/orphan.sh"
    write_hook_coverage "foo"

    run "$SCRIPT" "$FAKE_REPO"
    [ "$status" -eq 1 ]
    [[ "$output" == *"hooks with no test coverage"* ]]
    [[ "$output" == *"orphan"* ]]
}

@test "Bypass via AGENTOPS_PARITY_GATE_DISABLED" {
    echo '#!/usr/bin/env bash' > "$FAKE_REPO/hooks/orphan.sh"
    write_hook_coverage

    AGENTOPS_PARITY_GATE_DISABLED=1 run "$SCRIPT" "$FAKE_REPO"
    [ "$status" -eq 0 ]
    [[ "$output" == *"skipped"* ]]
}
