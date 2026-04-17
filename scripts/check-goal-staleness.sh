#!/usr/bin/env bash
# Checks GOALS.yaml for stale goals with simple grep-based checks that pass trivially.
#
# A goal is "stale" if its `added` date is older than 90 days.
# A goal has a "simple grep pattern" check if it uses only basic file inspection
# commands (grep, test -f, head, tail, wc, xargs test, tr) and does NOT invoke
# shell scripts (.sh), go test, go build, go vet, go test -race, ./tests/, ./scripts/.
#
# Flagged candidates (stale + simple check + passes) are printed to stderr.
# Exits non-zero if there are >5 flagged candidates.
#
# Usage: ./scripts/check-goal-staleness.sh
# Requires: yq (YAML processor), date (macOS compatible)
#
# GOALS.md migration (see CLAUDE.md §Agent Goals): this repo converted
# GOALS.yaml -> GOALS.md, so machine-readable `added:` / `check:` fields are no
# longer present. If GOALS.yaml is absent, the script skips with exit 0 rather
# than erroring, because no downstream gate depends on its output and erroring
# misleads readers about a real problem.

set -uo pipefail

# macOS-compatible cutoff date: 90 days ago as YYYY-MM-DD
CUTOFF_DATE=$(date -v-90d +%Y-%m-%d 2>/dev/null)
if [ -z "$CUTOFF_DATE" ]; then
  # GNU date fallback
  CUTOFF_DATE=$(date -d "90 days ago" +%Y-%m-%d 2>/dev/null)
fi

if [ -z "$CUTOFF_DATE" ]; then
  echo "ERROR: Could not compute cutoff date. Ensure macOS or GNU date is available." >&2
  exit 1
fi

GOALS_FILE="${1:-GOALS.yaml}"

if [ ! -f "$GOALS_FILE" ]; then
  # Post-GOALS.md migration: GOALS.yaml is the legacy YAML format. When it is
  # absent we skip because this script has no way to apply its staleness
  # heuristics against the current narrative markdown format.
  if [ -f "GOALS.md" ]; then
    echo "SKIP: GOALS.yaml not found; repo uses GOALS.md (post-migration). Nothing to check." >&2
    exit 0
  fi
  echo "ERROR: GOALS.yaml not found at $GOALS_FILE" >&2
  exit 1
fi

# is_simple_grep_check <check_string>
# Returns 0 (true) if the check is a simple grep/file-inspection pattern.
# Returns 1 (false) if the check invokes scripts or go commands.
is_simple_grep_check() {
  local check="$1"

  # Reject if the check contains any of these patterns
  if echo "$check" | grep -qE '\.(sh)(\s|$|"|'"'"')'; then
    return 1
  fi
  if echo "$check" | grep -qE '\bgo (test|build|vet)\b'; then
    return 1
  fi
  if echo "$check" | grep -qE '\./tests/'; then
    return 1
  fi
  if echo "$check" | grep -qE '\./scripts/'; then
    return 1
  fi
  # Reject bash -c wrappers that could contain complex logic
  if echo "$check" | grep -qE '^bash\s+-c\s+|^bash\s+"'; then
    return 1
  fi
  # Reject "cd && go ..." or "cd ... && go ..." patterns
  if echo "$check" | grep -qE 'cd\s+\S+\s+&&\s+(go|npm|python|ruby|cargo)'; then
    return 1
  fi
  # Reject node, jq -e (structured queries), for loops (complex multi-step)
  if echo "$check" | grep -qE '\bnode\b|\bjq\s+-e\b|\bfor\s+\w+\s+in\b|\bawk\b'; then
    return 1
  fi

  # Accept: must only contain allowed primitives
  # grep, test, head, tail, wc, xargs test, tr, &&, ||, |, quotes, paths, -q/-c/-r flags
  # We confirm by checking it does NOT use any shell execution beyond these
  # (Already checked above for disallowed commands)
  return 0
}

# date_is_older_than_cutoff <date_string>
# Returns 0 if date_string < CUTOFF_DATE (i.e. goal is stale/old)
date_is_older_than_cutoff() {
  local goal_date="$1"

  # Validate date format YYYY-MM-DD
  if ! echo "$goal_date" | grep -qE '^[0-9]{4}-[0-9]{2}-[0-9]{2}$'; then
    return 1
  fi

  # Lexicographic comparison works for ISO 8601 dates
  if [[ "$goal_date" < "$CUTOFF_DATE" ]]; then
    return 0
  fi
  return 1
}

FLAGGED_COUNT=0
FLAGGED_IDS=()

# Get the count of goals
GOAL_COUNT=$(yq e '.goals | length' "$GOALS_FILE" 2>/dev/null)

if [ -z "$GOAL_COUNT" ] || [ "$GOAL_COUNT" -eq 0 ]; then
  echo "No goals found in $GOALS_FILE" >&2
  exit 0
fi

for i in $(seq 0 $((GOAL_COUNT - 1))); do
  # Extract fields for this goal
  GOAL_ID=$(yq e ".goals[$i].id" "$GOALS_FILE" 2>/dev/null)
  ADDED=$(yq e ".goals[$i].added" "$GOALS_FILE" 2>/dev/null)
  CHECK=$(yq e ".goals[$i].check" "$GOALS_FILE" 2>/dev/null)

  # Skip goals without an `added` field (yq returns "null" for missing fields)
  if [ -z "$ADDED" ] || [ "$ADDED" = "null" ]; then
    continue
  fi

  # Skip goals without a check
  if [ -z "$CHECK" ] || [ "$CHECK" = "null" ]; then
    continue
  fi

  # Only process goals older than 90 days
  if ! date_is_older_than_cutoff "$ADDED"; then
    continue
  fi

  # Only process simple grep-pattern checks
  if ! is_simple_grep_check "$CHECK"; then
    continue
  fi

  # Run the check and see if it passes (exit 0 = trivially true candidate)
  if bash -c "$CHECK" >/dev/null 2>&1; then
    FLAGGED_COUNT=$((FLAGGED_COUNT + 1))
    FLAGGED_IDS+=("$GOAL_ID")
    echo "TRIVIALLY TRUE CANDIDATE: $GOAL_ID (added: $ADDED, check: $CHECK)" >&2
  fi
done

if [ "$FLAGGED_COUNT" -gt 0 ]; then
  echo "" >&2
  echo "Summary: $FLAGGED_COUNT trivially-true stale goal(s) flagged." >&2
  echo "These goals were added >90 days ago and their simple checks pass — they may need raising or retiring." >&2
else
  echo "No trivially-true stale goals found." >&2
fi

if [ "$FLAGGED_COUNT" -gt 5 ]; then
  exit 1
fi

exit 0
