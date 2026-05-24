# Gate Checks

> Extracted from implement SKILL.md Steps 0a-0b. Ratchet gate checks and pre-mortem validation prerequisites.

## Ratchet Status Check (RPI Workflow)

**Before implementation, verify prior workflow gates passed:**

```bash
# Check if ao CLI is available
if command -v ao &>/dev/null; then
  # Check if research and plan phases completed
  RATCHET_STATUS=$(ao ratchet status --json 2>/dev/null || echo '{}')
  RESEARCH_DONE=$(echo "$RATCHET_STATUS" | jq -r '.research.completed // false')
  PLAN_DONE=$(echo "$RATCHET_STATUS" | jq -r '.plan.completed // false')

  if [ "$RESEARCH_DONE" = "true" ] && [ "$PLAN_DONE" = "true" ]; then
    echo "Ratchet: Prior gates passed (research + plan complete)"
  elif [ "$RESEARCH_DONE" = "false" ] || [ "$PLAN_DONE" = "false" ]; then
    echo "WARNING: Prior gates not complete. Run /research and /plan first."
    echo "  Research: $RESEARCH_DONE"
    echo "  Plan: $PLAN_DONE"
    echo ""
    echo "Override with: ao ratchet skip <gate> --reason 'manual override'"
  fi

  # Get current spec path for reference
  SPEC_PATH=$(ao ratchet spec 2>/dev/null || echo "")
  if [ -n "$SPEC_PATH" ]; then
    echo "Ratchet: Current spec at $SPEC_PATH"
  fi
else
  echo "Ratchet: ao CLI not available - skipping gate check"
fi
```

**Fallback:** If ao is not available, proceed without ratchet checks. The skill continues normally.

## Pre-Flight Pre-Mortem Gate

**Before starting implementation, check if pre-mortem validation was run on the plan:**

```bash
if command -v ao &>/dev/null; then
  RATCHET_JSON=$(ao ratchet status --json 2>/dev/null || echo '{}')
  PRE_MORTEM_STATUS=$(echo "$RATCHET_JSON" | jq -r '.steps[]? | select(.name == "pre-mortem") | .status // "none"')
  PLAN_EXISTS=$(ls .agents/plans/*.md 2>/dev/null | head -1)

  if [ "$PRE_MORTEM_STATUS" = "pending" ] && [ -n "$PLAN_EXISTS" ]; then
    echo "Pre-mortem hasn't been run on your plan."
    echo "Options:"
    echo "  1. Run /pre-mortem first"
    echo "  2. Skip: ao ratchet skip pre-mortem --reason 'user chose to skip'"
    echo "  3. Proceed anyway"
    # Ask user: "Pre-mortem hasn't been run on your plan. Run /pre-mortem first, skip, or proceed?"
    # If skip: ao ratchet skip pre-mortem --reason "user chose to skip"
  fi
  # If ao unavailable or no chain: proceed silently
fi
```

**Fallback:** If ao is not available or no ratchet chain exists, proceed silently.

## Behavioral Spec Completeness (Advisory — Pre-Commit)

**When:** After Step 5b quality loop passes, before Step 6 commit.
**Severity:** WARN (advisory, not blocking). Blocking in `--quality` mode.

Check if the implementing agent generated a behavioral spec in Step 5c:

```bash
ISSUE_ID="${ISSUE_ID:-unknown}"
SPEC_FILE=".agents/specs/${ISSUE_ID}.json"

if [ -f "$SPEC_FILE" ]; then
  # Validate spec structure
  python3 -c "
import json, sys
with open('$SPEC_FILE') as f:
    spec = json.load(f)
assert 'id' in spec, 'Missing id'
assert spec['id'].startswith('auto-'), 'Agent spec id must start with auto-'
assert 'acceptance_vectors' in spec, 'Missing acceptance_vectors'
assert len(spec['acceptance_vectors']) >= 2, 'Need at least 2 acceptance vectors'
for v in spec['acceptance_vectors']:
    assert 'dimension' in v and 'threshold' in v, 'Vector missing dimension or threshold'
print('Spec gate: PASS')
" 2>&1 || echo "WARN: Spec validation failed — review .agents/specs/${ISSUE_ID}.json"
else
  echo "WARN: No behavioral spec generated at ${SPEC_FILE}"
  echo "  Step 5c was skipped or spec not written."
  echo "  This is advisory — commit proceeds without spec."
fi
```

**Skip conditions:** `--no-spec` flag, issue type is `docs`/`chore`/`ci`.
