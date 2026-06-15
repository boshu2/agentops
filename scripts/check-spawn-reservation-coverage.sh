#!/usr/bin/env bash
# check-spawn-reservation-coverage.sh — flag atm-spawned workers that are NOT
# born into coordination (ag-xe6u5, slice 3 of the ag-tixgy spawn gateway).
#
# The gateway (`atm spawn --reserve`) makes each worker register in Agent Mail
# AND hold a file reservation. A worker that is registered (recently active) but
# holds NO reservation is uncoordinated — it can silently collide on a hot file
# (the #1 swarm failure mode). This check surfaces that drift.
#
# Posture: WARN-ONLY by default (exit 0 with a warning). Pass --strict (or
# AGENTOPS_SPAWN_COVERAGE_STRICT=1) to exit non-zero when any recently-active
# agent holds no reservation. SKIPS cleanly (exit 0) when Agent Mail is absent —
# this is an operability aid, never a hard blocker on a box without AM.
#
# Recency: only agents active within --active-within minutes (default 30) are
# considered "live workers"; historical/dead agents from prior sessions are
# ignored so the signal stays about the current swarm.
#
# Test injection: set AGENTS_JSON / RESERVATIONS_JSON to file paths to feed
# fixtures instead of calling `am` (used by the bats suite).
#
# Exit codes:
#   0 = all live workers reserved, OR warn-only, OR AM absent (skip)
#   1 = --strict and >=1 live worker holds no reservation
#   2 = bad usage / unreadable fixture
set -uo pipefail

STRICT="${AGENTOPS_SPAWN_COVERAGE_STRICT:-0}"
ACTIVE_WITHIN=30
PROJECT="${PWD}"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --strict) STRICT=1; shift ;;
        --active-within) ACTIVE_WITHIN="$2"; shift 2 ;;
        --project) PROJECT="$2"; shift 2 ;;
        -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
        *) echo "unknown arg: $1" >&2; exit 2 ;;
    esac
done

warn() { echo "spawn-coverage: $*" >&2; }

# Resolve the agents + reservations JSON: fixtures win (tests), else live am.
read_agents() {
    if [[ -n "${AGENTS_JSON:-}" ]]; then
        cat "$AGENTS_JSON" 2>/dev/null || return 1
    elif command -v am >/dev/null 2>&1; then
        am robot agents --project "$PROJECT" 2>/dev/null || return 1
    else
        return 1
    fi
}
read_reservations() {
    if [[ -n "${RESERVATIONS_JSON:-}" ]]; then
        cat "$RESERVATIONS_JSON" 2>/dev/null || return 1
    elif command -v am >/dev/null 2>&1; then
        am robot reservations --project "$PROJECT" 2>/dev/null || return 1
    else
        return 1
    fi
}

if ! command -v jq >/dev/null 2>&1; then
    warn "jq not found — skipping (operability aid, not a hard gate)"
    exit 0
fi

agents_json="$(read_agents)" || { warn "Agent Mail unavailable — skipping coverage check"; exit 0; }
res_json="$(read_reservations)" || { warn "Agent Mail reservations unavailable — skipping"; exit 0; }

# last_active recency predicate: "just now"/"now"/"Ns ago" always live;
# "Nm ago" live iff N <= ACTIVE_WITHIN; "h"/"d"/"w" ago = stale.
# Emits the names of LIVE agents only.
live_agents="$(printf '%s' "$agents_json" | jq -r --argjson within "$ACTIVE_WITHIN" '
    .agents[]? as $a
    | ($a.last_active // "") as $la
    | (
        if ($la | test("just now|^now$|ago.*second|second.*ago|^\\d+s ago$"; "i")) then true
        elif ($la | test("^(\\d+)m ago$")) then ((($la | capture("^(?<n>\\d+)m ago$").n) | tonumber) <= $within)
        else false end
      ) as $islive
    | select($islive) | $a.name
')"

if [[ -z "$live_agents" ]]; then
    echo "spawn-coverage: no live workers (active within ${ACTIVE_WITHIN}m) — nothing to check"
    exit 0
fi

# Set of agents holding >=1 active reservation.
reserved_agents="$(printf '%s' "$res_json" | jq -r '.all_active[]?.agent' | sort -u)"

uncoordinated=()
while IFS= read -r agent; do
    [[ -z "$agent" ]] && continue
    if ! printf '%s\n' "$reserved_agents" | grep -qxF "$agent"; then
        uncoordinated+=("$agent")
    fi
done <<< "$live_agents"

if [[ ${#uncoordinated[@]} -eq 0 ]]; then
    echo "spawn-coverage: PASS — every live worker holds a reservation (born into coordination)"
    exit 0
fi

warn "${#uncoordinated[@]} live worker(s) registered but holding NO reservation (uncoordinated):"
for a in "${uncoordinated[@]}"; do warn "  - $a"; done
warn "Spawn with 'atm spawn --reserve <scope>' so workers are born into coordination (ag-tixgy)."

if [[ "$STRICT" == "1" ]]; then
    exit 1
fi
exit 0
