#!/usr/bin/env bash
# SessionEnd heavy maintenance (serialized across sessions).
# Runs batch-feedback, vibe-check, and maturity maintenance under a shared lock.

# Kill switch
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0

ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
ROOT="$(cd "$ROOT" 2>/dev/null && pwd -P 2>/dev/null || printf '%s' "$ROOT")"
AO_DIR="$ROOT/.agents/ao"
HOOK_ERROR_LOG="$AO_DIR/hook-errors.log"
LOCK_FILE="$AO_DIR/session-end-heavy.lock"
VIBECHECK_DIR="$ROOT/.agents/vibecheck"

mkdir -p "$AO_DIR" 2>/dev/null || true
mkdir -p "$VIBECHECK_DIR" 2>/dev/null || true

AO_TIMEOUT_BIN="timeout"
command -v "$AO_TIMEOUT_BIN" >/dev/null 2>&1 || AO_TIMEOUT_BIN="gtimeout"

log_hook_fail() {
    local message="$1"
    echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) HOOK_FAIL: ${message}" >> "$HOOK_ERROR_LOG" 2>/dev/null || true
}

run_ao_quick() {
    local seconds="$1"
    shift
    if command -v "$AO_TIMEOUT_BIN" >/dev/null 2>&1; then
        "$AO_TIMEOUT_BIN" "$seconds" ao "$@" 2>/dev/null
        return $?
    fi
    ao "$@" 2>/dev/null
}

run_maintenance_locked() {
    command -v ao >/dev/null 2>&1 || return 0

    run_ao_quick "${AGENTOPS_BATCH_FEEDBACK_TIMEOUT:-8}" \
        batch-feedback \
        --days "${AGENTOPS_BATCH_FEEDBACK_DAYS:-2}" \
        --max-sessions "${AGENTOPS_BATCH_FEEDBACK_MAX_SESSIONS:-3}" \
        --max-runtime "${AGENTOPS_BATCH_FEEDBACK_MAX_RUNTIME:-8s}" \
        --reward "${AGENTOPS_BATCH_FEEDBACK_REWARD:-0.70}" || log_hook_fail "ao batch-feedback"

    run_ao_quick "${AGENTOPS_VIBECHECK_TIMEOUT:-20}" \
        vibe-check --json --repo "$ROOT" > "$VIBECHECK_DIR/$(date -u +%Y-%m-%dT%H%M%SZ).json" || log_hook_fail "ao vibe-check"

    if [ "${AGENTOPS_EVICTION_DISABLED:-0}" != "1" ]; then
        run_ao_quick "${AGENTOPS_MATURITY_TIMEOUT:-4}" maturity --expire --archive || log_hook_fail "ao maturity --expire"
        run_ao_quick "${AGENTOPS_MATURITY_TIMEOUT:-4}" maturity --evict --archive || log_hook_fail "ao maturity --evict"
    fi
}

# Serialize heavy SessionEnd maintenance across processes.
if command -v flock >/dev/null 2>&1; then
    exec 9>"$LOCK_FILE" || exit 0
    flock -n 9 || exit 0
    run_maintenance_locked
    exit 0
fi

# Fallback lock when flock is unavailable.
LOCK_DIR="${LOCK_FILE}.d"
if mkdir "$LOCK_DIR" 2>/dev/null; then
    trap 'rmdir "$LOCK_DIR" 2>/dev/null || true' EXIT
    run_maintenance_locked
fi

exit 0
