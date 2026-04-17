#!/usr/bin/env bash
# check-goal-quality.sh — Rejects goals whose `check` field matches anti-patterns.
#
# Anti-patterns detected:
#   1. Any grep on PRODUCT.md (circular: we write the doc, then grep it to "verify").
#   2. Grep on .go source files for identifiers (redundant: build/vet catches missing code).
#   3. Grep on GOALS.yaml itself (circular: goal checks the goal file).
#
# NOT flagged (these are legitimate):
#   - README.md concept checks (verifies documentation keeps required messaging)
#   - SKILL.md behavior checks (verifies skills document required behaviors)
#   - Hook/config file checks (verifies wiring)
#   - Checks that run scripts, tests, or use jq validation
#
# Usage: ./scripts/check-goal-quality.sh [GOALS.yaml]
# Exit 0 = no flagged goals; Exit 1 = one or more flagged goals.

set -uo pipefail

GOALS_FILE="${1:-GOALS.yaml}"

if [[ ! -f "$GOALS_FILE" ]]; then
  # Post-GOALS.md migration (see CLAUDE.md §Agent Goals): the narrative
  # markdown format does not carry machine-readable `check:` fields, so the
  # anti-pattern detection here has nothing to run against. Skip gracefully
  # so that the script is safe to run locally without yelling misconfigured.
  if [[ -f "GOALS.md" ]]; then
    echo "SKIP: GOALS.yaml not found; repo uses GOALS.md (post-migration). Nothing to check." >&2
    exit 0
  fi
  echo "ERROR: $GOALS_FILE not found" >&2
  exit 2
fi

FLAGGED=0

goal_count=$(yq '.goals | length' "$GOALS_FILE")

for (( i=0; i<goal_count; i++ )); do
  id=$(yq ".goals[$i].id" "$GOALS_FILE")
  check=$(yq ".goals[$i].check" "$GOALS_FILE")

  # Skip goals with no check field
  if [[ -z "$check" || "$check" == "null" ]]; then
    continue
  fi

  flagged_reason=""

  # Anti-pattern 1: grep on PRODUCT.md (circular — we control this file)
  # Only flag when grep is the verb acting on PRODUCT.md, not awk/yq/wc/test
  if echo "$check" | grep -qE '\bgrep\b[^|]*PRODUCT\.md'; then
    flagged_reason="greps PRODUCT.md (circular: verifying content we wrote)"
  fi

  # Anti-pattern 2: Grep on .go source files for identifiers
  # Pattern: grep -q 'SomeIdentifier' path/to/file.go
  # These are redundant — if the identifier is removed, go build/vet fails first.
  # Exclude checks that also run go test/build/vet (those are meaningful).
  if [[ -z "$flagged_reason" ]]; then
    if echo "$check" | grep -qE "grep.*['\"].*['\"].*\.go\b|grep.*\.go\b"; then
      # Only flag if the check doesn't also run go test/build/vet or scripts
      if ! echo "$check" | grep -qE 'go (test|build|vet)|\.sh\b|./tests/|./scripts/'; then
        flagged_reason="greps .go file for identifier (redundant: build catches missing code)"
      fi
    fi
  fi

  # Anti-pattern 3: grep on GOALS.yaml itself (circular)
  # Only flag when grep is the verb acting on GOALS.yaml, not yq/awk
  if [[ -z "$flagged_reason" ]]; then
    if echo "$check" | grep -qE '\bgrep\b[^|]*GOALS\.yaml'; then
      flagged_reason="greps GOALS.yaml (circular: goal checks the goal file)"
    fi
  fi

  if [[ -n "$flagged_reason" ]]; then
    echo "FLAGGED [$id]: $flagged_reason" >&2
    echo "  check: $check" >&2
    FLAGGED=$((FLAGGED + 1))
  fi
done

if [[ "$FLAGGED" -eq 0 ]]; then
  exit 0
else
  echo "---" >&2
  echo "check-goal-quality: $FLAGGED flagged goal(s) found" >&2
  exit 1
fi
