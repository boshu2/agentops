#!/usr/bin/env bats

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
}

@test "pre-push wiring smoke performs a real push through the hook" {
    run "$REPO_ROOT/scripts/check-pre-push-gate-wired.sh" --dry-run-smoke
    [ "$status" -eq 0 ]
    [[ "$output" == *"git push invokes pre-push gate (sandbox smoke)"* ]]
    [[ "$output" == *"pre-push hook forces single-pass push gate"* ]]
    [[ "$output" == *"pre-push hook replays refspec stdin to bd"* ]]
    [[ "$output" == *"git push transmits through pre-push hook (sandbox smoke)"* ]]
}
