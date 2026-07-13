# Gate Checks

> Extracted from implement SKILL.md Steps 0a-0b. Ratchet gate checks and premortem validation prerequisites.

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
    echo "WARNING: Prior gates not complete. Run $research and $plan first."
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

## Pre-Flight Premortem Gate

**Before starting implementation, check if premortem validation was run on the plan:**

```bash
if command -v ao &>/dev/null; then
  RATCHET_JSON=$(ao ratchet status --json 2>/dev/null || echo '{}')
  PREMORTEM_STATUS=$(echo "$RATCHET_JSON" | jq -r '.steps[]? | select(.name == "premortem") | .status // "none"')
  PLAN_EXISTS=$(ls .agents/plans/*.md 2>/dev/null | head -1)

  if [ "$PREMORTEM_STATUS" = "pending" ] && [ -n "$PLAN_EXISTS" ]; then
    echo "Premortem hasn't been run on your plan."
    echo "Options:"
    echo "  1. Run $premortem first"
    echo "  2. Skip: ao ratchet skip premortem --reason 'user chose to skip'"
    echo "  3. Proceed anyway"
    # Ask user: "Premortem hasn't been run on your plan. Run $premortem first, skip, or proceed?"
    # If skip: ao ratchet skip premortem --reason "user chose to skip"
  fi
  # If ao unavailable or no chain: proceed silently
fi
```

**Fallback:** If ao is not available or no ratchet chain exists, proceed silently.
