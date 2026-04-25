#!/usr/bin/env bash
set -euo pipefail
[[ "${AGENTOPS_HOOKS_DISABLED:-}" == "1" ]] && exit 0

INPUT=$(cat)
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty' 2>/dev/null)

# Block reads to .agents/holdout/ for non-evaluator agents
if [[ "$TOOL_NAME" == "Read" || "$TOOL_NAME" == "Glob" || "$TOOL_NAME" == "Grep" ]]; then
    TARGETS=$(echo "$INPUT" | jq -r '
        [
            .tool_input.file_path?,
            .tool_input.path?,
            .tool_input.pattern?,
            .tool_input.glob?,
            .tool_input.query?
        ]
        | map(select(type == "string" and length > 0))
        | .[]
    ' 2>/dev/null)
    if [[ "$TARGETS" == *".agents/holdout"* ]] && [[ "${AGENTOPS_HOLDOUT_EVALUATOR:-}" != "1" ]]; then
        echo '{"decision":"block","reason":"Holdout scenarios are isolated from implementing agents. Set AGENTOPS_HOLDOUT_EVALUATOR=1 for evaluator access."}'
        exit 2
    fi
fi

# Block Bash commands targeting holdout directory
if [[ "$TOOL_NAME" == "Bash" ]]; then
    COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // empty' 2>/dev/null)
    if [[ "$COMMAND" == *".agents/holdout"* ]] && [[ "${AGENTOPS_HOLDOUT_EVALUATOR:-}" != "1" ]]; then
        echo '{"decision":"block","reason":"Holdout scenarios are isolated from implementing agents. Set AGENTOPS_HOLDOUT_EVALUATOR=1 for evaluator access."}'
        exit 2
    fi
fi

exit 0
