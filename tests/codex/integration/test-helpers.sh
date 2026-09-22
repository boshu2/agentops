#!/usr/bin/env bash
# Test-only setup and diagnostic capture for the three live Codex CLI probes.
# This file is sourced; run-all.sh enumerates probes explicitly.
CODEX_TEST_REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
# shellcheck source=tests/lib/run-with-timeout.sh
source "$CODEX_TEST_REPO_ROOT/tests/lib/run-with-timeout.sh"

codex_test_setup() {
    local name="$1"
    local log_root="${CODEX_TEST_LOG_DIR:-${TMPDIR:-/tmp}}"
    mkdir -p "$log_root"
    CODEX_TEST_ARTIFACTS="$(mktemp -d "$log_root/codex-$name.XXXXXX")"
    echo "Diagnostics: $CODEX_TEST_ARTIFACTS"
    if ! command -v codex >/dev/null 2>&1; then
        echo "SKIP: Codex CLI unavailable" >&2
        printf '77\n' > "$CODEX_TEST_ARTIFACTS/command.status"
        exit 77
    fi
    # shellcheck disable=SC2034 # Sourced probes consume this argument array.
    CODEX_TEST_MODEL_ARGS=()
    if [[ -n "${CODEX_MODEL:-}" ]]; then
        # shellcheck disable=SC2034 # Sourced probes consume this argument array.
        CODEX_TEST_MODEL_ARGS=(-c "model=\"$CODEX_MODEL\"" -c "review_model=\"$CODEX_MODEL\"")
    fi
    CODEX_TEST_PROJECT="$CODEX_TEST_ARTIFACTS/project"
    mkdir "$CODEX_TEST_PROJECT"
    git -C "$CODEX_TEST_PROJECT" init -q
    git -C "$CODEX_TEST_PROJECT" config user.email "fixture@example.invalid"
    git -C "$CODEX_TEST_PROJECT" config user.name "Codex Test Fixture"
}

codex_test_run() {
    local status
    if run_with_timeout "${CODEX_TEST_TIMEOUT_SECONDS:-120}" "Codex $1" \
        "$CODEX_TEST_ARTIFACTS/command.log" "${@:2}"; then
        status=0
    else
        status=$?
    fi
    printf '%s\n' "$status" > "$CODEX_TEST_ARTIFACTS/command.status"
    if [[ "$status" -ne 0 ]]; then
        if [[ "$status" -eq 124 ]]; then
            echo "FAIL: Codex timed out (exit 124)" >&2
        else
            echo "FAIL: Codex command failed (exit $status)" >&2
        fi
        tail -40 "$CODEX_TEST_ARTIFACTS/command.log" >&2
    fi
    return "$status"
}

codex_test_require_output() {
    if [[ ! -s "$1" ]]; then
        echo "FAIL: output missing or empty: $1" >&2
        return 1
    fi
}
