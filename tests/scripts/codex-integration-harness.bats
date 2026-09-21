#!/usr/bin/env bats
# Fake CLI witnesses for capture/status correctness, never live provider proof.
setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    PROBES="$REPO_ROOT/tests/codex/integration"
    mkdir -p "$BATS_TEST_TMPDIR/bin" "$BATS_TEST_TMPDIR/logs"
    export CODEX_TEST_LOG_DIR="$BATS_TEST_TMPDIR/logs"
    export CODEX_TEST_TIMEOUT_SECONDS=10
    export PATH="$BATS_TEST_TMPDIR/bin:$PATH"
    export FAKE_CODEX_MODE=success
    cat > "$BATS_TEST_TMPDIR/bin/codex" <<'FAKE'
#!/usr/bin/env bash
set -eu
mode="${FAKE_CODEX_MODE:-success}"
case "$mode" in
    nonzero) echo 'SKIPPED is diagnostic text, not a status' >&2; exit 23 ;;
    exit77) echo 'provider returned 77' >&2; exit 77 ;;
    timeout) echo 'timeout diagnostic retained' >&2; sleep 30; exit 0 ;;
    missing) exit 0 ;;
esac
if [[ "$1" == review ]]; then
    echo 'Review completed for the controlled fixture.'
    exit 0
fi
output=''
schema=''
while [[ $# -gt 0 ]]; do
    case "$1" in
        -o) output="$2"; shift 2 ;;
        --output-schema) schema="$2"; shift 2 ;;
        *) shift ;;
    esac
done
echo 'fake CLI diagnostic retained' >&2
if [[ -n "$schema" ]]; then
    [[ -s "$schema" ]] || exit 19
    case "$mode" in
        malformed) printf 'not JSON\n' > "$output" ;;
        wrong-type) printf '{"status":"ok","count":"3"}\n' > "$output" ;;
        extra) printf '{"status":"ok","count":3,"extra":true}\n' > "$output" ;;
        wrong-enum) printf '{"status":"bad","count":3}\n' > "$output" ;;
        missing-field) printf '{"status":"ok"}\n' > "$output" ;;
        stream) printf '{"status":"ok","count":3}\n{"status":"ok","count":3}\n' > "$output" ;;
        *) printf '{"status":"ok","count":3}\n' > "$output" ;;
    esac
else
    printf 'The fixture README describes one read-only probe file and requests no project changes.\n' > "$output"
fi
FAKE
    chmod +x "$BATS_TEST_TMPDIR/bin/codex"
}

@test "all three probes accept successful CLI fixtures and retain diagnostic status" {
    run bash "$PROBES/run-all.sh"
    [ "$status" -eq 0 ]
    [[ "$output" == *'3 passed, 0 failed, 0 unavailable'* ]]
    for name in review sandbox structured; do
        files=("$CODEX_TEST_LOG_DIR"/codex-"$name".*/command.status)
        [ "$(cat "${files[0]}")" = 0 ]
        [ -s "${files[0]%status}log" ]
    done
}

@test "nonzero native status and diagnostics survive SKIPPED text" {
    export FAKE_CODEX_MODE=nonzero
    run bash "$PROBES/run-all.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *'0 passed, 3 failed, 0 unavailable'* ]]
    for file in "$CODEX_TEST_LOG_DIR"/*/command.status; do
        [ "$(cat "$file")" = 23 ]
        grep -q 'SKIPPED is diagnostic text' "${file%status}log"
    done
}

@test "a present CLI returning 77 is failure, not unavailable" {
    export FAKE_CODEX_MODE=exit77
    run bash "$PROBES/run-all.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *'0 passed, 3 failed, 0 unavailable'* ]]
}

@test "timeout fails and retains timeout status and partial diagnostics" {
    export FAKE_CODEX_MODE=timeout CODEX_TEST_TIMEOUT_SECONDS=1
    run bash "$PROBES/test-codex-review.sh"
    [ "$status" -eq 124 ]
    [[ "$output" == *'timed out (exit 124)'* ]]
    files=("$CODEX_TEST_LOG_DIR"/*/command.status)
    [ "$(cat "${files[0]}")" = 124 ]
    grep -q 'timeout diagnostic retained' "${files[0]%status}log"
}

@test "successful exit without required output fails every probe without shell redirection errors" {
    export FAKE_CODEX_MODE=missing
    run bash "$PROBES/run-all.sh"
    [ "$status" -eq 1 ]
    [[ "$output" == *'0 passed, 3 failed, 0 unavailable'* ]]
    [[ "$output" == *'output missing or empty'* ]]
    [[ "$output" != *'No such file or directory'* ]]
    for file in "$CODEX_TEST_LOG_DIR"/*/command.status; do
        [ "$(cat "$file")" = 0 ]
    done
}

@test "complete schema validation rejects malformed JSON, wrong fields, types, enums and streams" {
    for mode in malformed wrong-type extra wrong-enum missing-field stream; do
        export FAKE_CODEX_MODE="$mode"
        run bash "$PROBES/test-structured-output.sh"
        [ "$status" -eq 1 ]
        [[ "$output" == *'does not satisfy structured-output.schema.json'* ]]
    done
}
