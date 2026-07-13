# Temporal Interrogation Framework

Walk through the implementation timeline to surface time-dependent risks that static plan review misses.

## Purpose

Plans look good on paper but fail in time. Temporal interrogation forces judges to simulate the implementation sequence hour by hour, exposing ordering dependencies, blocking resources, and compounding failures.

## Timeline Template

### Hour 1: Setup & First File

- What blocks the first meaningful code change?
- Are all dependencies available (APIs, credentials, packages)?
- Is the dev environment ready (DB migrations, seed data, config)?
- What happens if the first test fails?

### Hour 2: Core Implementation

- Which files must change in what order?
- Are there circular dependencies between changes?
- What's the longest uninterruptible sequence (can't save/test mid-way)?
- Where does the implementer need domain knowledge they might lack?

### Hour 4: Integration & Edge Cases

- What happens when components connect for the first time?
- Which error paths are untested until integration?
- Are there race conditions that only appear under load?
- What data shapes haven't been validated end-to-end?

### Hour 6+: Polish & Ship

- What's left that "should be quick" but historically isn't?
- Are docs, config updates, and migration scripts included?
- What manual verification is needed before merge?
- If the implementer is interrupted here and picks up tomorrow, what context is lost?

## Judge Prompt Addition

When temporal interrogation is enabled, add to each judge's prompt:

```
TEMPORAL INTERROGATION: Walk through this plan's implementation timeline.
For each phase (Hour 1, 2, 4, 6+), identify:
1. What blocks progress at this point?
2. What fails silently at this point?
3. What compounds if not caught at this point?
Report temporal findings in a separate "Timeline Risks" section.
```

## When to Use

- **Always for `--deep` reviews** — temporal interrogation is included automatically
- **On request** via `--temporal` flag for quick reviews
- **Auto-triggered** when plan has 5+ files or 3+ sequential dependencies

## Report Integration

Temporal findings appear in the premortem report as:

```markdown
## Timeline Risks

| Phase | Risk | Impact if Missed | Mitigation |
|-------|------|------------------|------------|
| Hour 1 | Missing API credentials | Blocks all progress | Add credential check to setup script |
| Hour 2 | Circular import between module A and B | Refactor needed mid-implementation | Extract shared types to common module first |
| Hour 4 | Race condition in parallel write path | Data corruption in production | Add mutex before integration testing |
| Hour 6+ | Migration script not tested on staging data | Rollback needed post-deploy | Run migration on staging clone first |
```

## Retro History Correlation

When `.agents/retro/index.jsonl` exists with 2+ entries, load the last 5 retros and check for recurring timeline-phase failures. If a phase (e.g., Hour 4 integration) has caused issues in 2+ prior retros, auto-escalate its severity in the current review.

```bash
if [ -f .agents/retro/index.jsonl ]; then
  RETRO_COUNT=$(wc -l < .agents/retro/index.jsonl)
  if [ "$RETRO_COUNT" -ge 2 ]; then
    echo "Retro history available — checking for recurring timeline risks"
    tail -5 .agents/retro/index.jsonl | jq -r '.footguns[]?' 2>/dev/null
  fi
fi
```
