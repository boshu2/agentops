# Track 2: Flywheel Proof

**Type:** Evolution cycle (RPI loop until done)
**Priority:** Secondary — overlaps with Track 1
**Thesis:** Create irrefutable evidence that sessions compound.

## Context

The core promise of AgentOps is "Make every coding agent session smarter than the last." This promise has never been mechanically validated end-to-end. Individual pieces work — learnings get created, injection happens at session start, knowledge gets retrieved — but nobody has run a scripted proof that demonstrates the full cycle: learn → inject → use → compound.

Without this proof:
- Users cannot see the value in their first session
- The compounding claim is a story, not a mechanical fact
- Regressions in the flywheel go undetected
- Adoption stalls because there is no "aha moment"

## Work Items

### 1. Proof Run Harness

Create `tests/fixtures/proof-repo/` — a simple Go module with 3-4 known issues (e.g., missing error handling, unused variable, missing test).

Create `tests/e2e/proof-run.sh` that simulates 3 sessions:

**Session 1: Discovery**
- Run `/research` (or equivalent) to discover issues
- Run `/learn` to capture 2 learnings about the codebase
- Assert: learning files created in `.agents/learnings/`
- Assert: learnings have valid frontmatter (id, type, created_at)

**Session 2: Compounding**
- Run `ao inject` to load learnings from Session 1
- Run `/rpi` (or equivalent) that uses injected knowledge
- Assert: injection happened (check inject output)
- Assert: output references or is influenced by prior learnings

**Session 3: Maturation**
- Run `ao inject` again
- Assert: knowledge maturation visible (freshness scores, retrieval counts)
- Assert: continued compounding (new learnings build on old)

The harness should be fully automated and runnable in CI.

### 2. System-Works Goal

Add to GOALS.yaml:

```yaml
- id: flywheel-proof-passes
  description: "Flywheel proof run demonstrates 3-session compounding"
  check: "bash tests/e2e/proof-run.sh"
  weight: 6
  pillar: knowledge-compounding
  added: "2026-02-21"
```

This is the most important product goal: "does the core promise actually work?"

### 3. Quickstart Flywheel Step

Add a Step 8 to the quickstart flow: "Prove the flywheel works."

After existing quickstart steps:
1. Run `ao inject` and show what was loaded
2. Demonstrate that the previous step's learning influences the next invocation
3. Show the "aha moment" — the session is smarter because of what came before

Update `skills/quickstart/SKILL.md` to include this step.

### 4. Progressive Disclosure

Reduce cognitive load for new users:

- Mark 35 of the 43 user-facing skills as `tier: advanced` or `tier: expert`
- Keep 8 visible starters: `/quickstart`, `/research`, `/council`, `/vibe`, `/rpi`, `/implement`, `/learn`, `/status`
- Update `skills/using-agentops/SKILL.md` to show only the 8 starters by default
- Remaining skills are still available but not shown in the default catalog

### 5. Goal Pruning Documentation

Update documentation to reflect the pruned goals:
- Update README.md goal count reference (if any) to match new 25-goal count
- Ensure `tests/docs/validate-goal-count.sh` passes with the new count
- Update any docs that reference the old 83-goal count

## Success Criteria

- [ ] `tests/fixtures/proof-repo/` exists with realistic toy codebase
- [ ] `tests/e2e/proof-run.sh` runs 3 sessions and all assertions pass
- [ ] `flywheel-proof-passes` goal added to GOALS.yaml and passing
- [ ] Quickstart SKILL.md has flywheel proof step
- [ ] Starter skill set reduced to 8 in using-agentops
- [ ] All existing tests continue to pass
- [ ] Proof run is runnable in CI (no interactive prompts)

## Approach

Run this as a `/evolve` cycle. The evolution loop should:
1. Create the proof repo fixture first (foundational)
2. Build the proof run harness script
3. Iterate until all 3 sessions pass their assertions
4. Add the goal and verify it passes
5. Update quickstart and progressive disclosure
6. Verify all existing tests still pass

## Key Files

```
tests/fixtures/proof-repo/         # Toy codebase for proof
tests/e2e/proof-run.sh             # 3-session proof script
skills/quickstart/SKILL.md         # Quickstart flow
skills/using-agentops/SKILL.md     # Skill catalog (progressive disclosure)
hooks/session-start.sh             # Knowledge injection entry point
lib/ao-inject.sh                   # Injection implementation
GOALS.yaml                         # Add flywheel-proof-passes goal
```

## Dependencies

- Track 1 (RPI stabilization) should be underway or complete before running the proof
  against real RPI cycles. However, the proof harness itself can be built in parallel.
- The proof run may use simplified versions of RPI phases rather than full `ao rpi phased`
  if the orchestrator is not yet stable enough.
