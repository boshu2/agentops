#!/usr/bin/env bash
# practices: [hexagonal-architecture, data-contracts, design-by-contract]
# Verify hook lease replacements point at real port interfaces.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GENERATOR="$REPO_ROOT/scripts/generate-hook-lease-inventory.py"

if [ "${AGENTOPS_HOOK_PORT_REPLACEMENTS_SKIP:-0}" = "1" ]; then
    echo "check-hook-port-replacements: SKIP (AGENTOPS_HOOK_PORT_REPLACEMENTS_SKIP=1)"
    exit 0
fi

declare -A PORT_FILES=(
    [CloseoutPort]="cli/internal/ports/closeout.go"
    [ContextCompilerPort]="cli/internal/ports/context_compiler.go"
    [EventBusPort]="cli/internal/ports/event_bus.go"
    [GateRunnerPort]="cli/internal/ports/gate_runner.go"
    [HarnessPort]="cli/internal/ports/harness.go"
    [OperatorPort]="cli/internal/ports/operator.go"
    [SafetyPolicyPort]="cli/internal/ports/safety_policy.go"
    [WorkspacePort]="cli/internal/ports/workspace.go"
)

declare -A INMEMORY_FILES=(
    [CloseoutPort]="cli/internal/ports/inmemory_closeout.go"
    [ContextCompilerPort]="cli/internal/ports/inmemory_context_compiler.go"
    [EventBusPort]="cli/internal/ports/inmemory_event_bus.go"
    [GateRunnerPort]="cli/internal/ports/inmemory_gate_runner.go"
    [HarnessPort]="cli/internal/ports/inmemory_harness.go"
    [OperatorPort]="cli/internal/ports/inmemory_operator.go"
    [SafetyPolicyPort]="cli/internal/ports/inmemory_safety_policy.go"
    [WorkspacePort]="cli/internal/ports/inmemory_workspace.go"
)

REQUIRED_PORTS=(
    CloseoutPort
    ContextCompilerPort
    EventBusPort
    GateRunnerPort
    HarnessPort
    SafetyPolicyPort
    WorkspacePort
)

for port in "${!PORT_FILES[@]}"; do
    if [ ! -f "$REPO_ROOT/${PORT_FILES[$port]}" ]; then
        echo "check-hook-port-replacements: FAIL - missing port file for $port: ${PORT_FILES[$port]}"
        exit 1
    fi
    if [ ! -f "$REPO_ROOT/${INMEMORY_FILES[$port]}" ]; then
        echo "check-hook-port-replacements: FAIL - missing in-memory adapter for $port: ${INMEMORY_FILES[$port]}"
        exit 1
    fi
done

json_tmp="$(mktemp)"
trap 'rm -f "$json_tmp"' EXIT
python3 "$GENERATOR" --format json --output "$json_tmp" || exit 1

declare -A SEEN_COUNTS=()
unknown_ports=()
while IFS= read -r port; do
    [ -z "$port" ] && continue
    if [ -z "${PORT_FILES[$port]+x}" ]; then
        unknown_ports+=("$port")
        continue
    fi
    SEEN_COUNTS[$port]=$(( ${SEEN_COUNTS[$port]:-0} + 1 ))
done < <(jq -r '.entries[] | select(.disposition != "remove") | .replacement' "$json_tmp" | grep -Eo '[A-Za-z]+Port' || true)

if [ "${#unknown_ports[@]}" -gt 0 ]; then
    echo "check-hook-port-replacements: FAIL - unknown port references in hook lease inventory:"
    printf '  - %s\n' "${unknown_ports[@]}"
    exit 1
fi

for port in "${REQUIRED_PORTS[@]}"; do
    if [ "${SEEN_COUNTS[$port]:-0}" -eq 0 ]; then
        echo "check-hook-port-replacements: FAIL - required port $port is not referenced by any non-remove hook lease"
        exit 1
    fi
done

echo "check-hook-port-replacements: PASS (${#REQUIRED_PORTS[@]} required hook replacement ports present)"
