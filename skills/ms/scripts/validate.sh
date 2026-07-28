#!/usr/bin/env bash
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKILL="$SKILL_DIR/SKILL.md"
MCP_SEARCH="$SKILL_DIR/scripts/mcp-search.py"

[[ -s "$SKILL" ]]
grep -q '^name: ms$' "$SKILL"
# Canonical source carries full metadata; the generated Codex package uses slim
# frontmatter but mirrors this validator and the runnable body. Effects must be
# declared HONESTLY: ms spawns an MCP search server, writes feedback/outcome rows
# to a live DB, and rebuilds the index. An empty effects list is a known
# falsehood, so require the real effect tokens rather than assert emptiness.
if grep -q '^metadata:' "$SKILL"; then
  if grep -q '^  effects: \[\]$' "$SKILL"; then
    echo 'ms effects must not be empty: it spawns a search server and writes feedback/outcome/index state' >&2
    exit 1
  fi
  grep -q 'spawn_search_server' "$SKILL"
  grep -q 'write_feedback_outcomes' "$SKILL"
  grep -q 'rebuild_search_index' "$SKILL"
fi
grep -Fq 'Keep `ms` retrieval-only for production skill work.' "$SKILL"
grep -Fq '**Authority boundary:** `skills/**` is canonical source' "$SKILL"
grep -Fq '**Outcome timing:** Record `ms outcome` only after the caller has independent evidence' "$SKILL"
grep -Fq 'A zero-result `ms search` CLI response is not evidence' "$SKILL"
grep -Fq 'python3 skills/ms/scripts/mcp-search.py "<query>"' "$SKILL"
if grep -Eiq 'pawl|AUTO-REDO|ONE-HELPER|circuit breaker|canonical factory|promotes a skill' "$SKILL"; then
  echo 'ms contract contains retired factory/lifecycle vocabulary' >&2
  exit 1
fi
[[ -s "$MCP_SEARCH" ]]
python3 "$MCP_SEARCH" --help >/dev/null

echo "ms retrieval contract: PASS"
