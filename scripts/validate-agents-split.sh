#!/usr/bin/env bash
# validate-agents-split.sh
#
# Enforces the lean AGENTS.md orientation contract after the root sibling
# cutover (docs authority migrate-then-delete for AGENTS-*).
#   - AGENTS.md exists and is <=250 lines (orientation only)
#   - AGENTS.md contains pointer links to the three detail owners
#   - Each owner exists and links back to AGENTS.md
#
# Owners:
#   docs/agent-workflow-reference.md
#   docs/CI-CD.md
#   docs/contracts/codex-skill-api.md
#
# Exit codes:
#   0 — contract satisfied
#   1 — contract broken
#   2 — usage / setup error

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

readonly LIMIT=250
readonly OWNERS=(
  docs/agent-workflow-reference.md
  docs/CI-CD.md
  docs/contracts/codex-skill-api.md
)

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

# 3. Each owner exists
for owner in "${OWNERS[@]}"; do
  checks=$((checks + 1))
  if [ ! -f "$owner" ]; then
    fail "missing owner: $owner"
  fi
done

# 4. AGENTS.md links to each owner (path substring is enough)
for owner in "${OWNERS[@]}"; do
  checks=$((checks + 1))
  if ! grep -q "$owner" AGENTS.md; then
    fail "AGENTS.md does not link to $owner"
  fi
done

# 5. Each owner back-links to AGENTS.md
for owner in "${OWNERS[@]}"; do
  [ -f "$owner" ] || continue
  checks=$((checks + 1))
  if ! grep -q "AGENTS.md" "$owner"; then
    fail "$owner does not back-link to AGENTS.md"
  fi
done

# 6. Root AGENTS-* siblings must be gone
for sib in AGENTS-WORKFLOW.md AGENTS-CI.md AGENTS-CODEX.md AGENTS-RUNTIME.md; do
  checks=$((checks + 1))
  if [ -e "$sib" ]; then
    fail "legacy root sibling still present: $sib (migrate content to owners and delete)"
  fi
done

echo "validate-agents-split: scanned $checks checks"

if [ "$failed" -eq 0 ]; then
  echo "PASS — AGENTS.md ($lines lines) + 3 detail owners, links bidirectional; no root siblings."
  exit 0
fi

echo "FAIL — $failed contract breach(es):" >&2
for f in "${failures[@]}"; do
  echo "  - $f" >&2
done
exit 1
