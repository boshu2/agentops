#!/usr/bin/env bats
# Tests for scripts/push-serial.sh (ag-qidx P1.4) — the within-host advisory
# lock. We exercise the lock-contention path, which exits before any git call.

setup() {
    ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    SCRIPT="$ROOT/scripts/push-serial.sh"
    export TMPDIR="$BATS_TEST_TMPDIR"
}

@test "push-serial.sh exists and is executable" {
    [ -x "$SCRIPT" ]
}

@test "times out when the lock is already held (contention path)" {
    # Pre-hold the lock so mkdir fails, and set timeout to 0 for an immediate
    # give-up. This path exits before any git call (no repo/network needed) and
    # before the EXIT-trap is armed, so the pre-held lock is left intact.
    mkdir "$BATS_TEST_TMPDIR/agentops-push.lock"
    run env PUSH_LOCK_TIMEOUT=0 bash "$SCRIPT" origin main
    [ "$status" -eq 1 ]
    [[ "$output" == *"timed out"* ]]
    [ -d "$BATS_TEST_TMPDIR/agentops-push.lock" ]
}
