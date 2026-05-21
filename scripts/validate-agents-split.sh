#!/usr/bin/env bash
# validate-agents-split.sh
#
# Enforces the tiered AGENTS.md split (soc-vuu6.3):
#   - AGENTS.md exists and is <=250 lines (orientation only)
#   - AGENTS-{WORKFLOW,CI,CODEX,RUNTIME}.md exist
#   - AGENTS.md contains a pointer link to each sibling
#   - Each sibling links back to AGENTS.md for orientation
#
# Why: AGENTS.md was 580 lines / ~15K tokens before the split (every session
# paid the cold-start cost). The split saves cost for routine sessions while
# keeping the operational detail one hop away. This gate keeps the split
# contract honest as the docs evolve.
#
# Sibling pattern: matches scripts/check-finding-registry.sh shape —
# explicit gate with summary + nonzero exit on first contract breach.
#
# Exit codes:
#   0 — split contract satisfied
#   1 — contract broken (one or more checks failed)
#   2 — usage / setup error

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

readonly LIMIT=250
readonly SIBLINGS=(AGENTS-WORKFLOW.md AGENTS-CI.md AGENTS-CODEX.md AGENTS-RUNTIME.md)

declare -i checks=0
declare -i failed=0
failures=()

fail() {
  failed=$((failed + 1))
  failures+=("$1")
}

# 1. AGENTS.md exists
checks=$((checks + 1))
if [ ! -f AGENTS.md ]; then
  fail "AGENTS.md does not exist"
  echo "validate-agents-split: scanned $checks checks, $failed failed" >&2
  printf '  - %s\n' "${failures[@]}" >&2
  exit 1
fi

# 2. AGENTS.md is <=LIMIT lines
checks=$((checks + 1))
lines=$(wc -l < AGENTS.md)
if [ "$lines" -gt "$LIMIT" ]; then
  fail "AGENTS.md is $lines lines, exceeds $LIMIT-line orientation budget"
fi

# 3. Each sibling exists
for sib in "${SIBLINGS[@]}"; do
  checks=$((checks + 1))
  if [ ! -f "$sib" ]; then
    fail "missing sibling: $sib"
  fi
done

# 4. AGENTS.md links to each sibling
for sib in "${SIBLINGS[@]}"; do
  checks=$((checks + 1))
  if ! grep -q "$sib" AGENTS.md; then
    fail "AGENTS.md does not link to $sib"
  fi
done

# 5. Each sibling back-links to AGENTS.md
for sib in "${SIBLINGS[@]}"; do
  [ -f "$sib" ] || continue
  checks=$((checks + 1))
  if ! grep -q "AGENTS.md" "$sib"; then
    fail "$sib does not back-link to AGENTS.md"
  fi
done

echo "validate-agents-split: scanned $checks checks"

if [ "$failed" -eq 0 ]; then
  echo "PASS — AGENTS.md ($lines lines) + 4 siblings, links bidirectional."
  exit 0
fi

echo "FAIL — $failed contract breach(es):" >&2
for f in "${failures[@]}"; do
  echo "  - $f" >&2
done
exit 1
