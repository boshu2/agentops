#!/usr/bin/env bash
# SessionEnd: forge learnings + maturity maintenance (serialized via lock).

# TEMP: disabled until pend-* triple-ID amplification is verifiably fixed.
# This script invokes `ao flywheel close-loop`, `ao session close --auto-extract`,
# and `ao forge transcript`, all of which can re-amplify the stale
# .agents/knowledge/pending/ queue into pend-*-pend-*-pend-* files in
# .agents/learnings/. See active-incident learning:
#   .agents/learnings/2026-04-30-pend-pollution-actively-growing-during-session.md
# Companion patch in hooks/ao-flywheel-close.sh.
exit 0

# Kill switch
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
PLUGIN_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
ROOT="$(cd "$ROOT" 2>/dev/null && pwd -P 2>/dev/null || printf '%s' "$ROOT")"
AO_DIR="$ROOT/.agents/ao"
LOCK_FILE="$AO_DIR/session-end-heavy.lock"

mkdir -p "$AO_DIR" 2>/dev/null || true

AO_TIMEOUT_BIN="timeout"
command -v "$AO_TIMEOUT_BIN" >/dev/null 2>&1 || AO_TIMEOUT_BIN="gtimeout"

run_ao_quick() {
    local seconds="$1"; shift
    if command -v "$AO_TIMEOUT_BIN" >/dev/null 2>&1; then
        "$AO_TIMEOUT_BIN" "$seconds" ao "$@" 2>/dev/null
        return $?
    fi
    ao "$@" 2>/dev/null
}

run_maintenance() {
    command -v ao >/dev/null 2>&1 || return 0

    # Step 0: Auto-extract learnings while context is fresh (before forge)
    # Note: session close does not support --quiet; removed to prevent silent failure
    run_ao_quick 10 session close --auto-extract || true

    FORGE_STATUS=0
    if run_ao_quick 6 forge transcript --last-session --quiet; then
        # Run the full flywheel close-loop: ingest → auto-promote → citation feedback
        # → maturity transitions → store index → memory promotion
        # Replaces separate pool ingest + maturity --scan --apply calls
        run_ao_quick 15 flywheel close-loop --quiet || true

        # Capture metric baseline for velocity trend tracking (golden signals)
        run_ao_quick 5 metrics baseline -o json || true

        # Sync to repo-root MEMORY.md (opt-in via AGENTOPS_MEMORY_SYNC=1)
        # Only runs after successful forge — no point syncing stale data
        if [ "${AGENTOPS_MEMORY_SYNC:-0}" = "1" ]; then
            run_ao_quick 10 memory sync --quiet || true
        fi
    else
        FORGE_STATUS=1
    fi

    # Write session outcome for binary reward signal (used by flywheel close-loop)
    OUTCOME_FILE="$AO_DIR/last-session-outcome.json"
    if [ "$FORGE_STATUS" -eq 0 ]; then
        printf '{"outcome":"success","written_at":"%s"}' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$OUTCOME_FILE" 2>/dev/null || true
    else
        printf '{"outcome":"failure","written_at":"%s"}' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$OUTCOME_FILE" 2>/dev/null || true
    fi

    # Knowledge maintenance: deduplicate and detect contradictions
    run_ao_quick 6 dedup --merge || true
    run_ao_quick 6 contradict || true

    if [ "${AGENTOPS_EVICTION_DISABLED:-0}" != "1" ]; then
        run_ao_quick 4 maturity --expire --archive || true
        run_ao_quick 4 maturity --evict --archive || true
        run_ao_quick 4 maturity --curate --archive || true
    fi

    # Always refresh compiled prevention outputs from the current finding ledger
    # before session close. Missing registry/artifacts fail open inside the helper.
    FINDING_COMPILER="${PLUGIN_ROOT}/hooks/finding-compiler.sh"
    if [ -x "$FINDING_COMPILER" ]; then
        if command -v "$AO_TIMEOUT_BIN" >/dev/null 2>&1; then
            "$AO_TIMEOUT_BIN" 20 bash "$FINDING_COMPILER" --quiet 2>/dev/null || true
        else
            bash "$FINDING_COMPILER" --quiet 2>/dev/null || true
        fi
    fi

    # Auto-prune .agents/ (opt-out via AGENTOPS_AUTO_PRUNE=0)
    if [ "${AGENTOPS_AUTO_PRUNE:-1}" = "1" ]; then
        PRUNE_SCRIPT="${PLUGIN_ROOT}/scripts/prune-agents.sh"
        if [ -x "$PRUNE_SCRIPT" ]; then
            "$AO_TIMEOUT_BIN" 15 bash "$PRUNE_SCRIPT" --execute --quiet 2>/dev/null || true
        fi
    fi
}

# Serialize across processes
if command -v flock >/dev/null 2>&1; then
    exec 9>"$LOCK_FILE" || exit 0
    flock -n 9 || exit 0
    run_maintenance
    exit 0
fi

# Fallback lock
LOCK_DIR="${LOCK_FILE}.d"
if mkdir "$LOCK_DIR" 2>/dev/null; then
    trap 'rmdir "$LOCK_DIR" 2>/dev/null || true' EXIT
    run_maintenance
fi

exit 0
