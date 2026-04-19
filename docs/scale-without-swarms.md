# Scale Without Swarms

> 3-5 focused agents with fresh context beat 60 stale ones every time.

The hot take in agent orchestration is scale: more agents, faster delivery. Claude-Flow and similar systems advertise 60, 80, even 100+ simultaneous agents as a feature. AgentOps takes the opposite bet. Here's why.

---

## The Problem With Agent Swarms

Massive swarms sound compelling until you watch them in production.

**Context pollution.** Each agent in a large swarm accumulates context from prior tasks, prior failures, prior teammates' decisions. By the time agent-47 starts its task, its context window is half-full of irrelevant history. Output quality degrades proportionally.

**Merge conflicts by construction.** 60 agents writing to the same codebase without coordination produce 60 conflicting branches. The integration tax — resolving conflicts, re-running tests, re-validating — often exceeds the work cost. You've parallelized the easy part and serialized the hard part.

**Redundant work.** Without dependency mapping, agents discover the same shared function needs changing and all edit it independently. 10 agents fix the same bug 10 ways. The last one to commit wins; the other 9 cycles are waste.

**No regression gates.** Speed-optimized swarms skip validation to maximize throughput. A gate that blocks one wave from starting until the prior wave passes would halve their advertised parallelism numbers. So they skip it — and ship regressions.

**The stale oracle problem.** A 60-agent swarm means 60 context windows that haven't seen what the other 59 just did. By cycle 10, every agent is operating on a stale mental model of the codebase. Decisions made on stale context produce fragile code.

---

## The AgentOps Model

AgentOps bets on quality per agent over count of agents.

The core insight: **agent output quality is a function of context quality.** Context quality degrades with size, staleness, and irrelevance. The solution is not more agents — it's tighter context control per agent.

The model has three components:

**Isolation:** Each worker gets fresh context for exactly its task. Nothing from prior waves bleeds in. Workers communicate through the filesystem, not accumulated chat history.

**Waves:** Work is dependency-mapped upfront. Wave 1 runs in parallel; when it passes gates, Wave 2 starts. Parallelism where it's safe; sequencing where it's required. The plan determines this, not the operator.

**Gates:** Every wave completes a regression check before the next wave begins. A wave that introduces regressions doesn't proceed — it fails, the operator sees why, and the system stops rather than compounding the failure.

The result: 3-5 workers per wave, fresh context, gated progress. Not 60 workers racing to completion with no safety net.

---

## Ralph Wiggum Pattern

The core isolation mechanism is the [Ralph Wiggum Pattern](https://ghuntley.com/ralph/): each execution unit starts fresh, as if it has no memory of what came before.

Named after Ralph Wiggum's cheerful cluelessness — each worker starts fresh with no memory of previous workers. This sounds like a weakness. It's the mechanism that makes everything else work.

**Fresh context per worker.** Workers don't inherit stale context. They get a precise context bundle: their task, the relevant code, injected learnings from the knowledge flywheel, nothing else.

**Disk-backed state.** State that must survive between cycles lives on disk in `.agents/` — not in LLM memory, not in accumulated chat context that gets compacted away. The cycle state is always recoverable. A worker failing or a context compaction event doesn't lose progress.

**Knowledge flywheel.** What workers learn *does* persist — but through a curated pipeline. Session-end hooks mine the transcript for learnings, score them (specificity, actionability, novelty, confidence), and write them to the flywheel. The next wave gets those learnings injected at start. Cycle 50 knows what cycle 1 learned the hard way. But it knows it cleanly, scored by freshness, not as accumulated chat noise.

---

## Wave Execution

`/plan` decomposes a goal into issues and maps their dependencies. The result is a wave structure: which issues can run in parallel, which must wait on others.

`/crank` executes that structure:

```
Wave 1: [issue-1, issue-2, issue-3] → parallel → gate
Wave 2: [issue-4, issue-5]          → parallel → gate
Wave 3: [issue-6]                   → serial   → gate
```

**Parallel within waves.** Issues in the same wave have no dependencies between them — they can safely run concurrently. Workers don't step on each other.

**Sequential between waves.** Wave N+1 starts only after Wave N passes its gate. Dependencies are respected automatically; no manual coordination required.

**Automatic dependency resolution.** The operator specifies the goal, not the execution order. `/plan` derives the order from declared dependencies. The system handles coordination; the operator handles the roadmap.

---

## Worktree Isolation

Each worker runs in its own git worktree — a clean checkout of the current HEAD, separate from every other worker's filesystem.

The effect: no merge conflicts during execution. Workers write to isolated filesystems. The lead reviews their output and commits. Conflicts, if any, are resolved at the lead-commit step — not scattered across 60 concurrent branches.

This is not defensive programming. It's the construction of the system: **parallelism is safe because shared state doesn't exist during worker execution.**

No shared mutable state + fresh context = parallel execution that scales without coordination overhead.

---

## Regression Gates

Every wave is gated. No wave proceeds until the prior wave passes.

The gate mechanism:

1. Worker completes task, writes output to `.agents/`
2. Lead validates output: does it pass the wave's acceptance criteria?
3. Fitness snapshot taken: do all GOALS.yaml checks still pass?
4. If yes: commit, proceed to next wave
5. If no: stop, surface the failure, do not proceed

This is not advisory. The gate is hard. A regression that would have been caught at step 3 doesn't get buried under 5 more waves of changes. It's visible, isolated, and fixable before it becomes a multi-wave debugging exercise.

`/evolve` extends this to cycle-level: every cycle's fitness score is written to `cycle-history.jsonl`. A cycle that regresses a previously-passing goal auto-reverts and halts. The floor can never drop.

---

## The Numbers

One `/evolve` run on this repo: **116 cycles, ~7 hours, unattended, zero regressions.**

What it shipped:
- Test coverage: ~85% → ~97% across 203 files
- Complex functions (cyclomatic complexity >= 8): dozens → zero
- Modern Go idioms: sentinel errors, exhaustive switches, Go 1.26-compatible slices/cmp.Or/range-over-int
- 132 commits, each traceable, each regression-gated

No human intervention during the run. Every cycle picked the worst remaining gap by weight, ran `/rpi` to fix it, validated nothing regressed, extracted learnings, and looped.

Compare to a 60-agent swarm running for 7 hours without gates: you'd have parallel branches, unresolved conflicts, and an unknown regression surface to debug in the morning. The overnight run with gates produces a codebase you trust. The overnight swarm produces a codebase you have to audit.

---

## When to Scale

Worker count is a tunable, not a maximization target.

**Start with 1 worker per issue in a wave.** Dependencies drive wave structure. Let the plan tell you how many workers a wave needs, not the other way around.

**3-5 workers per wave is typical.** Enough parallelism to make waves fast; small enough that the lead's validation step stays tractable.

**Swarm mode for embarrassingly parallel work.** `/swarm` is the right tool when tasks have no dependencies — research across multiple domains, brainstorming approaches, independent file analysis. Swarm is not the default; it's the tool for specific shapes of work.

**Sequential for high-risk changes.** When changes are cross-cutting, when the codebase is unfamiliar, when the gate needs to be tight — run sequential waves, even waves of one. The regression gate catches problems before they compound. Speed isn't the constraint; confidence is.

**The answer to "should I use more agents?" is almost always "tighten the context and gates first."** A well-constrained plan with 4 workers and hard gates will outperform an unconstrained swarm of 40 every time.

---

*See also: [PRODUCT.md](https://github.com/boshu2/agentops/blob/main/PRODUCT.md) — Orchestration at Scale value proposition, Roadmap*
