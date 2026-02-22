#!/usr/bin/env bash
# Log a triage decision for accuracy tracking
#
# Usage: ./scripts/log-triage-decision.sh <file:line> <tool> <verdict> <agent>
#
# Arguments:
#   file:line   - Location of the finding (e.g., src/auth.go:42)
#   tool        - Tool that found the issue (e.g., semgrep, gitleaks, ruff)
#   verdict     - Triage verdict: TRUE_POS or FALSE_POS
#   agent       - Agent that made the decision (e.g., security-reviewer)
#
# Output: Appends JSONL entry to $TOOLCHAIN_OUTPUT_DIR/triage-decisions.jsonl (default: $TMPDIR/agentops-tooling/)
#
# Ground truth can be added later via:
#   jq '. | select(.file_line == "src/auth.go:42") | .ground_truth = "TRUE_POS"'

set -euo pipefail

FILE_LINE="${1:-}"
TOOL="${2:-}"
VERDICT="${3:-}"  # TRUE_POS, FALSE_POS
AGENT="${4:-}"

if [[ -z "$FILE_LINE" || -z "$TOOL" || -z "$VERDICT" || -z "$AGENT" ]]; then
    echo "Usage: $0 <file:line> <tool> <verdict> <agent>"
    echo ""
    echo "Arguments:"
    echo "  file:line   Location of finding (e.g., src/auth.go:42)"
    echo "  tool        Tool name (e.g., semgrep, gitleaks)"
    echo "  verdict     TRUE_POS or FALSE_POS"
    echo "  agent       Agent that triaged (e.g., security-reviewer)"
    exit 1
fi

# Validate verdict
if [[ "$VERDICT" != "TRUE_POS" && "$VERDICT" != "FALSE_POS" ]]; then
    echo "Error: verdict must be TRUE_POS or FALSE_POS, got: $VERDICT"
    exit 1
fi

TOOLING_DIR="${TOOLCHAIN_OUTPUT_DIR:-${TMPDIR:-/tmp}/agentops-tooling}"
LOG_FILE="$TOOLING_DIR/triage-decisions.jsonl"
mkdir -p "$TOOLING_DIR"

jq -n \
  --arg file_line "$FILE_LINE" \
  --arg tool "$TOOL" \
  --arg verdict "$VERDICT" \
  --arg agent "$AGENT" \
  --arg timestamp "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  '{file_line: $file_line, tool: $tool, verdict: $verdict, agent: $agent, timestamp: $timestamp, ground_truth: null}' \
  >> "$LOG_FILE"

echo "Logged: $FILE_LINE ($TOOL) -> $VERDICT by $AGENT"
