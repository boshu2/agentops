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

## Flywheel Section

**ALWAYS include the flywheel section** (regardless of `--spawn-next` flag):

```markdown
## Flywheel: Next Cycle

Post-mortem harvested N follow-up items (M process-improvements, K tech-debt):

| # | Title | Type | Severity |
|---|-------|------|----------|
| 1 | ... | process-improvement | high |

Ready to run:
    /rpi "<highest-severity item title>"
```

The `--spawn-next` flag controls whether items are **marked consumed** in `next-work.jsonl`. The suggestion is ALWAYS shown. This ensures every `/rpi` cycle ends by pointing at the next one -- the flywheel never stops spinning unless there's nothing to improve.
