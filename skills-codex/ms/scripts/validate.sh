#!/usr/bin/env bash
set -euo pipefail

SKILL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKILL="$SKILL_DIR/SKILL.md"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# Exact Markdown contract literals: backticks are data, never substitutions.
# shellcheck disable=SC2016
MARKERS=(
  '## Output Specification'
  '- **Validation command:** run `skills/ms/scripts/validate.sh` for the production lifecycle contract and `scripts/ms-reindex.sh --check-source` for normalized source equivalence.'
  '- **Downstream handoff:** the invoking agent consumes full loaded guidance, routes production skill intent to the canonical factory, and records `ms outcome` only after downstream use and validation—not after retrieval alone.'
  '## Production Skill Handoff'
  '**Production-intent handoff:** When the query or intended use is to create, edit, heal, or promote a skill, `ms` only retrieves full guidance and then routes execution to `agentops-skill-factory`, `skill-builder`, `heal-skill`, and the factory-selected validation primitives.'
  '**Authority boundary:** `skills/**` is canonical source; the generator owns the `ms` Codex twin and other projections. Never edit the index, loaded copies, or generated projections as source.'
  '**Promotion gate:** Promotion requires deterministic checks plus a fresh-context pawl or independent verdict; the producing agent never self-certifies completion.'
  '**Failure routing:** A plain `REFUTED` verdict auto-repairs and revalidates. Only a tripped circuit breaker enters `HOLD` and receives exactly one bounded helper consultation before re-earning an independent verdict.'
  '**Outcome timing:** Record `ms outcome` only after the downstream factory use and validation complete, never after retrieval alone.'
  '## Quality Checklist'
  '- Production skill intent leaves `ms` after retrieval and enters the canonical factory against `skills/**`; generated twins and loaded/indexed copies are never hand-edited as source.'
  '- Promotion carries deterministic evidence and a fresh-context independent verdict; no producer self-certification is accepted.'
  '- `REFUTED` stays in automatic repair, while exactly one helper is reserved for a tripped breaker and `ms outcome` waits for downstream validation.'
)

validate_contract() {
  local file="$1" marker
  [[ -s "$file" ]] || return 1
  for marker in "${MARKERS[@]}"; do
    grep -Fqx -- "$marker" "$file" || return 1
  done
}

delete_one_negative_fixture() {
  local marker variant="$TMP/missing-marker.md"
  for marker in "${MARKERS[@]}"; do
    grep -Fvx -- "$marker" "$SKILL" >"$variant"
    if validate_contract "$variant"; then
      echo "ms lifecycle validator accepted a missing marker: $marker" >&2
      return 1
    fi
  done
}

validate_contract "$SKILL"
delete_one_negative_fixture
echo "ms lifecycle contract: PASS"
