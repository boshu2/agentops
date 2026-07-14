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
- Discovery: .agents/rpi/execution-packet.json
- Crank: <canonical tranche/wave evidence paths from the packet>
- Validate: <canonical result.json path + digest>
- Learn: <canonical learn-receipt.json path + digest>
- Next Work: .agents/rpi/next-work.jsonl
```

Legacy phase summaries may be listed only as link-only compatibility
projections. Do not restate their findings in this report.

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
