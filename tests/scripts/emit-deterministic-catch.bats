#!/usr/bin/env bats
# Tests for scripts/emit-deterministic-catch.sh (age-srl) — the deterministic
# membrane tier: when a pre-push Go gate BLOCKS, record the catch as a REFUTED
# gate-verdict so the in-situ catch-rate stops undercounting.
#
# L2: the green case drives the REAL ao binary end-to-end (emit -> ao yield
# gauge counts the REFUTED), proving the catch lands where the gauge reads it.

setup_file() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    # Resolve a usable ao binary once: prefer an env override, then the
    # conventional build output, else build into the file tmpdir.
    if [ -n "${AO_TEST_BIN:-}" ] && [ -x "${AO_TEST_BIN}" ]; then
        AO_BIN_RESOLVED="$AO_TEST_BIN"
    elif [ -x "$REPO_ROOT/cli/bin/ao" ]; then
        AO_BIN_RESOLVED="$REPO_ROOT/cli/bin/ao"
    else
        AO_BIN_RESOLVED="$BATS_FILE_TMPDIR/ao"
        ( cd "$REPO_ROOT/cli" && go build -o "$AO_BIN_RESOLVED" ./cmd/ao ) || AO_BIN_RESOLVED=""
    fi
    export REPO_ROOT AO_BIN_RESOLVED
}

setup() {
    SCRIPT="$REPO_ROOT/scripts/emit-deterministic-catch.sh"
    # A throwaway project root that is its own git repo, so the ao yield ledger
    # resolves (by CWD) into here and never touches the real ledger.
    WORK="$BATS_TEST_TMPDIR/proj"
    mkdir -p "$WORK"
    (
        cd "$WORK"
        git init -q
        git config user.email t@t && git config user.name t
        echo seed > seed.txt && git add . && git commit -qm "seed ag-test1 work"
    )
    LEDGER="$WORK/.agents/yield/yield-ledger.jsonl"
}

@test "script exists and is executable" {
    [ -f "$SCRIPT" ]
    [ -x "$SCRIPT" ]
}

@test "green: a blocked push emits a REFUTED that ao yield gauge counts" {
    [ -n "$AO_BIN_RESOLVED" ] || skip "no ao binary available"
    run bash -c "cd '$WORK' && AO_BIN='$AO_BIN_RESOLVED' bash '$SCRIPT'"
    [ "$status" -eq 0 ]
    [ -f "$LEDGER" ]
    grep -q '"event":"gate-verdict"' "$LEDGER"
    grep -q 'REFUTED' "$LEDGER"
    grep -q 'deterministic' "$LEDGER"
    # The gauge must read it as a real catch: catch_rate = 1/(1+0) = 1.
    run bash -c "cd '$WORK' && '$AO_BIN_RESOLVED' yield gauge --run deterministic-pre-push --json"
    [ "$status" -eq 0 ]
    [[ "$output" == *'"catch_rate": 1'* ]]
}

@test "fail-open: missing ao binary never blocks (exit 0, no ledger)" {
    run bash -c "cd '$WORK' && AO_BIN=/nonexistent/ao PATH=/usr/bin:/bin bash '$SCRIPT'"
    [ "$status" -eq 0 ]
    [ ! -f "$LEDGER" ]
}

@test "skip flag disables the emit (exit 0, no ledger)" {
    [ -n "$AO_BIN_RESOLVED" ] || skip "no ao binary available"
    run bash -c "cd '$WORK' && AGENTOPS_DETERMINISTIC_CATCH_SKIP=1 AO_BIN='$AO_BIN_RESOLVED' bash '$SCRIPT'"
    [ "$status" -eq 0 ]
    [ ! -f "$LEDGER" ]
}

@test "AO_YIELD_RUN_ID override routes the verdict to that run bucket" {
    [ -n "$AO_BIN_RESOLVED" ] || skip "no ao binary available"
    run bash -c "cd '$WORK' && AO_YIELD_RUN_ID=myrun AO_BIN='$AO_BIN_RESOLVED' bash '$SCRIPT'"
    [ "$status" -eq 0 ]
    grep -q '"run_id":"myrun"' "$LEDGER"
}
