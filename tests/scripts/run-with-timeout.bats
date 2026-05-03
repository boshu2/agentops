#!/usr/bin/env bats
# run-with-timeout.bats - Tests for tests/lib/run-with-timeout.sh

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    HELPER="$REPO_ROOT/tests/lib/run-with-timeout.sh"
    TMP_DIR="$(mktemp -d)"
}

teardown() {
    rm -rf "$TMP_DIR"
}

@test "run_with_timeout returns success and captures command output" {
    log_file="$TMP_DIR/success.log"

    run bash -c 'source "$1"; run_with_timeout 5 success "$2" bash -c "echo ok"' _ "$HELPER" "$log_file"

    [ "$status" -eq 0 ]
    [ "$(cat "$log_file")" = "ok" ]
}

@test "run_with_timeout returns 124 on timeout and kills child process group" {
    script="$TMP_DIR/hangs.sh"
    marker="$TMP_DIR/child-survived"
    pid_file="$TMP_DIR/child.pid"
    log_file="$TMP_DIR/timeout.log"

    cat > "$script" <<'SCRIPT'
#!/usr/bin/env bash
(sleep 10; touch "$1") &
echo "$!" > "$2"
wait
SCRIPT
    chmod +x "$script"

    run bash -c 'source "$1"; run_with_timeout 1 hanging-lane "$2" bash "$3" "$4" "$5"' \
        _ "$HELPER" "$log_file" "$script" "$marker" "$pid_file"

    [ "$status" -eq 124 ]
    [[ "$(cat "$log_file")" == *"TIMEOUT: hanging-lane exceeded 1s"* ]]

    child_pid="$(cat "$pid_file")"
    sleep 1
    ! kill -0 "$child_pid" 2>/dev/null
    [ ! -e "$marker" ]
}

@test "run_with_timeout cleans up child process group after command failure" {
    script="$TMP_DIR/fails-with-child.sh"
    marker="$TMP_DIR/failed-child-survived"
    pid_file="$TMP_DIR/failed-child.pid"
    log_file="$TMP_DIR/failure.log"

    cat > "$script" <<'SCRIPT'
#!/usr/bin/env bash
(sleep 10; touch "$1") &
echo "$!" > "$2"
exit 7
SCRIPT
    chmod +x "$script"

    run bash -c 'source "$1"; run_with_timeout 5 failing-lane "$2" bash "$3" "$4" "$5"' \
        _ "$HELPER" "$log_file" "$script" "$marker" "$pid_file"

    [ "$status" -eq 7 ]

    child_pid="$(cat "$pid_file")"
    sleep 1
    ! kill -0 "$child_pid" 2>/dev/null
    [ ! -e "$marker" ]
}
