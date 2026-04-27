#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

assert_exit() {
    local label="$1" expected="$2" hook_path="$3" env_value="$4" payload="$5"
    local actual=0

    if [[ "$env_value" == "__unset__" ]]; then
        echo "$payload" | env -u AGENTOPS_HOLDOUT_EVALUATOR bash "$hook_path" >/dev/null 2>&1 || actual=$?
    else
        echo "$payload" | AGENTOPS_HOLDOUT_EVALUATOR="$env_value" bash "$hook_path" >/dev/null 2>&1 || actual=$?
    fi

    if [[ "$actual" -ne "$expected" ]]; then
        echo "$label: expected exit $expected, got $actual" >&2
        return 1
    fi
}

run_hook_matrix() {
    local hook_path="$1"

    assert_exit "non-holdout read" 0 "$hook_path" "__unset__" \
        '{"tool_name":"Read","tool_input":{"file_path":"src/main.go"}}'
    assert_exit "holdout read blocked" 2 "$hook_path" "__unset__" \
        '{"tool_name":"Read","tool_input":{"file_path":".agents/holdout/private.json"}}'
    assert_exit "holdout glob pattern blocked" 2 "$hook_path" "__unset__" \
        '{"tool_name":"Glob","tool_input":{"pattern":".agents/holdout/*.json"}}'
    assert_exit "holdout grep query blocked" 2 "$hook_path" "__unset__" \
        '{"tool_name":"Grep","tool_input":{"query":".agents/holdout/private.json"}}'
    assert_exit "holdout bash command blocked" 2 "$hook_path" "__unset__" \
        '{"tool_name":"Bash","tool_input":{"command":"cat .agents/holdout/private.json"}}'
    assert_exit "evaluator glob allowed" 0 "$hook_path" "1" \
        '{"tool_name":"Glob","tool_input":{"pattern":".agents/holdout/*.json"}}'

    local actual=0
    echo '{"tool_name":"Glob","tool_input":{"pattern":".agents/holdout/*.json"}}' \
        | AGENTOPS_HOOKS_DISABLED=1 bash "$hook_path" >/dev/null 2>&1 || actual=$?
    if [[ "$actual" -ne 0 ]]; then
        echo "kill switch: expected exit 0, got $actual" >&2
        return 1
    fi
}

run_hook_matrix "$ROOT/hooks/holdout-isolation-gate.sh"
run_hook_matrix "$ROOT/cli/embedded/hooks/holdout-isolation-gate.sh"

echo "holdout isolation behavior passed"
