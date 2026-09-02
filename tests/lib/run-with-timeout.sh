#!/usr/bin/env bash
# shellcheck shell=bash
# Run a command with a bounded timeout and clean up its process group.

[[ -n "${RUN_WITH_TIMEOUT_SH_LOADED:-}" ]] && return 0
RUN_WITH_TIMEOUT_SH_LOADED=1

run_with_timeout_cleanup() {
    local pid="${1:?pid required}"
    local grace="${2:-2}"

    if kill -TERM "-$pid" 2>/dev/null; then
        sleep "$grace"
        kill -KILL "-$pid" 2>/dev/null || true
    elif kill -TERM "$pid" 2>/dev/null; then
        sleep "$grace"
        kill -KILL "$pid" 2>/dev/null || true
    fi

    wait "$pid" 2>/dev/null || true
}

run_with_timeout() {
    local timeout_seconds="${1:?timeout required}"
    local label="${2:?label required}"
    local log_file="${3:?log file required}"
    shift 3

    if [[ ! "$timeout_seconds" =~ ^[0-9]+$ ]] || [[ "$timeout_seconds" -eq 0 ]]; then
        echo "FAIL: $label invalid timeout: $timeout_seconds" > "$log_file"
        return 2
    fi

    mkdir -p "$(dirname "$log_file")"
    : > "$log_file"

    local child_pid
    if command -v perl >/dev/null 2>&1; then
        perl -e 'setpgrp(0, 0); exec @ARGV or die "exec failed: $!\n"' -- "$@" > "$log_file" 2>&1 &
    else
        "$@" > "$log_file" 2>&1 &
    fi
    child_pid=$!

    local start now elapsed status
    start="$(date +%s)"

    while kill -0 "$child_pid" 2>/dev/null; do
        now="$(date +%s)"
        elapsed=$((now - start))
        if [[ "$elapsed" -ge "$timeout_seconds" ]]; then
            {
                echo ""
                echo "TIMEOUT: $label exceeded ${timeout_seconds}s; terminating child processes"
            } >> "$log_file"
            run_with_timeout_cleanup "$child_pid"
            return 124
        fi
        sleep 1
    done

    status=0
    wait "$child_pid" || status=$?

    if [[ "$status" -ne 0 ]]; then
        run_with_timeout_cleanup "$child_pid"
    fi

    return "$status"
}
