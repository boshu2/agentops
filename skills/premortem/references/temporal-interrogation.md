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
- **For a named temporal risk** such as a migration cutover, expiring credential,
  irreversible sequence, or coordination window. File and dependency counts do
  not select depth by themselves.

## Between-wave bounded mode

Do not run temporal interrogation after every wave. Reuse the bound Premortem
while acceptance, dependencies, write scope, and risk remain unchanged. When a
wave materially changes one of those inputs, the orchestrator sends the changed
plan for one fresh Premortem and interrogates only:

1. the next wave and its exact first failing proof;
2. write-scope or dependency changes caused by the completed wave;
3. new risks or invalidated assumptions in the wave evidence; and
4. whether the next leaf still has one owner and a safe discard path.

Do not resimulate completed waves or rerun their deterministic/semantic proof.
Emit one bounded PASS/FAIL artifact for the exact changed plan. Premortem does
not own the repair count or next transition; the orchestrator reads the complete
blocker set and decides whether to repair or replan.

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

## Optional history correlation for deep mode

In `--deep` mode only, a cited, directly relevant prior failure may inform the
review. Do not scan a broad history index on the routine path, and never
auto-escalate severity solely from recurrence counts.
