#!/usr/bin/env bash
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKILL="$SKILL_DIR/SKILL.md"

[[ -s "$SKILL" ]]
grep -q '^name: ms$' "$SKILL"
grep -q '^  effects: \[\]$' "$SKILL"
grep -Fq 'Keep `ms` retrieval-only for production skill work.' "$SKILL"
grep -Fq '**Authority boundary:** `skills/**` is canonical source' "$SKILL"
grep -Fq '**Outcome timing:** Record `ms outcome` only after the caller has independent evidence' "$SKILL"
! grep -Eiq 'pawl|AUTO-REDO|ONE-HELPER|circuit breaker|canonical factory|promotes a skill' "$SKILL"

echo "ms retrieval contract: PASS"
