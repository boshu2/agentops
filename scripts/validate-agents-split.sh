#!/usr/bin/env bash
# Compatibility-named gate for the compact AGENTS contract and canonical routes.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

readonly LIMIT=250
readonly ROUTES=(
  docs/agent-workflow-reference.md
  docs/CI-CD.md
  docs/contracts/codex-skill-api.md
  docs/contracts/repo-execution-profile.md
)

checks=0
failed=0
failures=()
fail() { failed=$((failed + 1)); failures+=("$1"); }

checks=$((checks + 1))
if [[ ! -f AGENTS.md ]]; then
  fail "AGENTS.md does not exist"
  lines=0
else
  lines="$(wc -l <AGENTS.md)"
  checks=$((checks + 1))
  (( lines <= LIMIT )) || fail "AGENTS.md is $lines lines, exceeds $LIMIT-line operating-contract budget"
fi

for route in "${ROUTES[@]}"; do
  checks=$((checks + 1))
  [[ -s "$route" ]] || fail "missing or empty canonical route: $route"
done

echo "validate-agents-split: scanned $checks checks"
if (( failed == 0 )); then
  echo "PASS — AGENTS.md ($lines lines) + ${#ROUTES[@]} canonical on-demand routes."
  exit 0
fi

echo "FAIL — $failed contract breach(es):" >&2
printf '  - %s\n' "${failures[@]}" >&2
exit 1
