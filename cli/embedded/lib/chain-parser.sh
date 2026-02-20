#!/bin/bash
# chain-parser.sh — Shared chain.jsonl parsing for hooks
# Handles dual-schema: CANONICAL (gate+status) and LEGACY (step+locked)

# chain_find_entry CHAIN_FILE STEP_NAME
# Returns the last matching line for step_name, searching both "gate" and "step" fields
chain_find_entry() {
    local chain_file="$1" step_name="$2"
    grep -E "\"(step|gate)\"[[:space:]]*:[[:space:]]*\"${step_name}\"" "$chain_file" 2>/dev/null | tail -1
}

# chain_is_done ENTRY_LINE
# Returns 0 (true) if entry is locked/skipped/done, 1 (false) otherwise
# Handles both schemas: CANONICAL "status":"locked|skipped" and LEGACY "locked":true
chain_is_done() {
    local entry="$1"
    [ -z "$entry" ] && return 1
    if echo "$entry" | grep -qE '"status"[[:space:]]*:[[:space:]]*"(locked|skipped)"'; then
        return 0
    fi
    if echo "$entry" | grep -qE '"locked"[[:space:]]*:[[:space:]]*true'; then
        return 0
    fi
    return 1
}
