#!/usr/bin/env bash
set -euo pipefail

# validate-context-budget.sh — enforce the RPI context budget.
#
# Council .agents/council/2026-05-15-rpi-leanness-council.md: the RPI
# execution-packet (the runtime contract handed to a worker) must stay small.
# A bloated packet leaks the full plan into every worker's context window and
# defeats the smaller-loop thesis. This script is the mechanical gate.
#
# Usage:
#   scripts/validate-context-budget.sh [PACKET_PATH]
#       Check that the slice plan / execution-packet at PACKET_PATH (default
#       .agents/rpi/execution-packet.json) is within the 8 KB budget.
#       Exit 0 if within budget, exit 1 if it exceeds 8192 bytes.
#
#   scripts/validate-context-budget.sh --worker-prompt PATH [PACKET_PATH]
#       Also flag worker prompt files that embed a full plan document.
#       Heuristic: a worker prompt should hand off a *reference* to the plan,
#       not the plan itself. If PATH contains a line that starts a plan doc
#       section (## Files to Modify / ## Baseline Audit / ## Execution Order),
#       the prompt is treated as carrying an embedded plan and the check fails.
#
#   scripts/validate-context-budget.sh --self-test
#       Build temp fixtures (one >8 KB, one <=8 KB), assert the budget check
#       FAILs on the oversized one and PASSes on the small one, clean up, and
#       exit 0 only if both assertions hold. This drives the acceptance gate.
#
# CI parity (finding-2026-05-07-ci-parity-as-wave-acceptance): this script uses
# only POSIX/coreutils (wc, grep, mktemp) and no host-specific paths, so it
# behaves identically locally and in CI.

BUDGET_BYTES=8192
PREFIX="CONTEXT_BUDGET"

# Plan-document section headings. A worker prompt that contains any of these
# at the start of a line is assumed to embed a full plan rather than reference
# one. Kept deliberately simple and documented (see council note above).
PLAN_DOC_HEADING_RE='^## (Files to Modify|Baseline Audit|Execution Order)'

fail() {
    echo "$PREFIX: FAIL: $*" >&2
}

# file_size <path> — print byte count using only coreutils.
file_size() {
    wc -c < "$1" | tr -d ' '
}

# check_budget <path> — exit 0 if within budget, 1 if over (or missing).
check_budget() {
    local path="$1"
    if [[ ! -f "$path" ]]; then
        fail "execution-packet not found: $path"
        return 1
    fi
    local size
    size="$(file_size "$path")"
    if (( size > BUDGET_BYTES )); then
        fail "execution-packet $path is $size bytes, over the ${BUDGET_BYTES}-byte budget"
        return 1
    fi
    echo "$PREFIX: PASS: $path is $size bytes (<= ${BUDGET_BYTES}-byte budget)"
    return 0
}

# check_worker_prompt <path> — exit 1 if the prompt embeds a full plan doc.
check_worker_prompt() {
    local path="$1"
    if [[ ! -f "$path" ]]; then
        fail "worker prompt not found: $path"
        return 1
    fi
    local hits
    hits="$(grep -nE "$PLAN_DOC_HEADING_RE" "$path" || true)"
    if [[ -n "$hits" ]]; then
        fail "worker prompt $path embeds a full plan document (hand off a reference instead): ${hits//$'\n'/; }"
        return 1
    fi
    echo "$PREFIX: PASS: worker prompt $path does not embed a plan document"
    return 0
}

# --- --self-test mode ---
self_test() {
    local tmpdir big small rc_big rc_small
    tmpdir="$(mktemp -d)"
    # shellcheck disable=SC2064
    trap "rm -rf '$tmpdir'" EXIT

    big="$tmpdir/oversized-packet.json"
    small="$tmpdir/lean-packet.json"

    # >8 KB fixture: emit a single block well over the budget. Using printf in
    # a loop avoids the `yes | head` SIGPIPE that trips `set -o pipefail`.
    : > "$big"
    local i
    for (( i = 0; i < ((BUDGET_BYTES + 4096) / 64); i++ )); do
        printf '%064d\n' "$i" >> "$big"
    done
    # <=8 KB fixture: a small, realistic packet stub.
    printf '{"slice":"demo","budget":"lean"}\n' > "$small"

    local big_size small_size
    big_size="$(file_size "$big")"
    small_size="$(file_size "$small")"

    rc_big=0
    check_budget "$big" >/dev/null 2>&1 || rc_big=$?
    rc_small=0
    check_budget "$small" >/dev/null 2>&1 || rc_small=$?

    local ok=true
    if (( rc_big == 0 )); then
        fail "self-test: oversized fixture ($big_size bytes) should FAIL the budget check but passed"
        ok=false
    fi
    if (( rc_small != 0 )); then
        fail "self-test: lean fixture ($small_size bytes) should PASS the budget check but failed"
        ok=false
    fi

    if [[ "$ok" != true ]]; then
        echo "$PREFIX: SELF-TEST FAILED" >&2
        exit 1
    fi
    echo "$PREFIX: SELF-TEST PASS (oversized=$big_size bytes -> FAIL as expected; lean=$small_size bytes -> PASS as expected)"
    exit 0
}

# --- argument parsing ---
WORKER_PROMPT=""
PACKET_PATH=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --self-test)
            self_test
            ;;
        --worker-prompt)
            if [[ $# -lt 2 ]]; then
                fail "--worker-prompt requires a path argument"
                exit 1
            fi
            WORKER_PROMPT="$2"
            shift 2
            ;;
        -h|--help)
            grep -E '^#( |$)' "${BASH_SOURCE[0]}" | sed -E 's/^# ?//'
            exit 0
            ;;
        --*)
            fail "unknown option: $1"
            exit 1
            ;;
        *)
            PACKET_PATH="$1"
            shift
            ;;
    esac
done

PACKET_PATH="${PACKET_PATH:-.agents/rpi/execution-packet.json}"

errors=0
check_budget "$PACKET_PATH" || errors=$((errors + 1))

if [[ -n "$WORKER_PROMPT" ]]; then
    check_worker_prompt "$WORKER_PROMPT" || errors=$((errors + 1))
fi

if (( errors > 0 )); then
    exit 1
fi

echo "$PREFIX: PASS"
