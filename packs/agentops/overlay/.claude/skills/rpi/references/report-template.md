# Final Report Template

After all phases complete, summarize the entire lifecycle to the user.

## Summary Report

```markdown
## /rpi Complete

**Goal:** <goal>
**Epic:** <epic-id>
**Cycle:** <rpi_state.cycle> (parent: <rpi_state.parent_epic or "none">)

| Phase | Verdict/Status |
|-------|---------------|
| Research | Complete |
| Plan | Complete (<N> issues, <M> waves) |
| Pre-mortem | <PASS/WARN/FAIL> |
| Crank | <DONE/BLOCKED/PARTIAL> |
| Vibe | <PASS/WARN/FAIL> |
| Post-mortem | Complete |

**Artifacts:**
- Research: .agents/research/...
- Plan: .agents/plans/...
- Pre-mortem: .agents/council/...
- Vibe: .agents/council/...
- Post-mortem: .agents/council/...
- Learnings: .agents/learnings/...
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
