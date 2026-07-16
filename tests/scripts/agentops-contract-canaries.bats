#!/usr/bin/env bats

setup() {
    ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$ROOT/scripts/test-agentops-contract-canaries.sh"
}

@test "contract canary runner is an explicit retired tombstone" {
    run "$SCRIPT"

    [ "$status" -eq 2 ]
    [[ "$output" == *"RETIRED: contract canaries depended on the removed ao eval surface"* ]]
}

@test "contract canary tombstone never invokes a supplied evaluator" {
    sentinel="$BATS_TEST_TMPDIR/evaluator-ran"
    ao="$BATS_TEST_TMPDIR/ao"
    cat > "$ao" <<SH
#!/usr/bin/env bash
touch "$sentinel"
SH
    chmod +x "$ao"

    run "$SCRIPT" --ao-bin "$ao"

    [ "$status" -eq 2 ]
    [ ! -e "$sentinel" ]
}
