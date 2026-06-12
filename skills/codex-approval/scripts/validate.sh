#!/usr/bin/env bash
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKILL="$SKILL_DIR/SKILL.md"
FAIL=0

require_grep() {
  local label="$1"
  local pattern="$2"
  if grep -qE "$pattern" "$SKILL"; then
    echo "PASS: $label"
  else
    echo "FAIL: $label"
    FAIL=1
  fi
}

test -f "$SKILL" || {
  echo "FAIL: missing SKILL.md"
  exit 1
}

require_grep "frontmatter name" '^name: codex-approval$'
require_grep "trigger marker" '^[[:space:]]*Triggers:'
require_grep "ATM/NTM validator lane" 'ATM/NTM'
require_grep "busy pane protection" 'busy pane'
require_grep "tmux capture contract" 'ntm-captures'
require_grep "council artifact contract" 'fable-approval'
require_grep "print-mode Claude prohibition" 'Do not run `claude -p` or'
require_grep "WARN handling" 'WARN is not a silent pass'
require_grep "PerspectivePlan contract" 'PerspectivePlan'
require_grep "SynthesisPacket contract" 'SynthesisPacket'
require_grep "ApprovalEdge contract" 'ApprovalEdge'

LINES="$(wc -l < "$SKILL" | tr -d ' ')"
if [ "$LINES" -le 250 ]; then
  echo "PASS: line count $LINES <= 250"
else
  echo "FAIL: line count $LINES > 250"
  FAIL=1
fi

if [ "$FAIL" -eq 0 ]; then
  echo "PASS: codex-approval skill is structurally sound"
  exit 0
fi

echo "FAIL: codex-approval skill validation failed"
exit 1
