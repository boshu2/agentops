# Final Report Template

After all phases complete, summarize the entire lifecycle to the user.

## Summary Report

```markdown
## /rpi Complete

**Goal:** <goal>
**Epic:** <epic-id>
**Cycle:** <rpi_state.cycle> (parent: <rpi_state.parent_epic or "none">)

| Umbrella | Verdict/Status |
|-------|---------------|
| Discovery | DONE |
| Crank | <DONE/BLOCKED/PARTIAL> |
| Validate | <PASS/WARN/FAIL> |
| Learn | <DONE/BLOCKED/PARTIAL> |

**Artifacts:**
- Discovery: .agents/rpi/phase-1-summary.md
- Crank: .agents/rpi/phase-2-summary.md
- Validate: .agents/rpi/phase-3-summary.md
- Learn: .agents/rpi/phase-4-summary.md
- Next Work: .agents/rpi/next-work.jsonl
```

## Learn Section

Always include the immutable verdict reference and Learn plan impact:

```markdown
## Learn: Plan Impact

- Verdict: <artifact + digest>
- Remaining work: <true|false>
- Disposition: <material_change|no_change|terminal>
- Orchestrator decision: <replan|retry|continue|stop|escalate|close>
- Changed-plan Premortem: <artifact or not-applicable>
```

Optional next-work suggestions remain advisory. They never replace the Learn
receipt or authorize a direct retry.
