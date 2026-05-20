# Long-Loop Discipline — Disk Is Truth, Conversation Is Decorative

The cross-cutting axiom that every other `/evolve` reference implements.
When a loop runs past ~50 cycles (`/evolve`, `/loop`, `ScheduleWakeup`-driven
cron), **every load-bearing state must live on disk, not in conversation
context.** The conversation accumulates: skill prompts re-inject, hook
reminders pile up, tool-result history grows. Auto-compact handles it well
enough that correctness survives — *if and only if* the work-state was on
disk to begin with.

This doc is the principle; the implementing references are listed below.

## The Axiom

| State | Truth surface |
|---|---|
| Cycle outcome | `.agents/evolve/cycle-history.jsonl` (append-only, read at every cycle start) |
| Hypothesis tracker | `.agents/evolve/hypotheses.jsonl` |
| Queueable work | `bd ready` (Dolt-backed, out-of-band) |
| Skill rules | `skills/<n>/SKILL.md` + `references/` (read fresh per cycle) |
| Conversation context | **Decorative** — re-derives state from disk; never trusted alone |

If you remove the conversation context entirely between cycles and the work
keeps moving, the loop is healthy. If removing it breaks the loop, you are
storing load-bearing state in the wrong place.

## Evidence (anchored)

> "Every cycle starts by recovering state from `.agents/evolve/cycle-history.jsonl`
> — not from the conversation. Context drift in the conversation doesn't lose
> the work-state … The conversation context that accumulates is mostly
> decorative; the load-bearing state is on disk."
— `docs/learnings/2026-05-13-loop-context-drift-87-cycle-observation.md`
(87-cycle empirical observation)

> "text in SKILL.md is aspirational until paired with harness automation."
— `docs/learnings/2026-05-11-evolve-skill-friction-from-13-cycle-session.md`
(H45.2 + H45.3 falsified within 4-6 cycles because the rule lived only in
prose, not in a gate)

> "Cycle 12's deep-scout … was the most productive move of cycles 11-13 even
> though it shipped zero code. … A scout cycle is neither productive (ships
> code) nor idle (finds nothing): it ships durable knowledge about pending
> work, narrowing future cycles' search and discovery cost."
— `.agents/learnings/2026-05-11-evolve-scout-mode-pattern.md`
(durable knowledge IS the disk-truth surface)

> "in a shared tree, commit by explicit path (`git add <files>`), never
> `git add -A`; treat full-repo gate runs as unreliable when other agents
> are active and lean on targeted proof commands instead."
— `.agents/learnings/2026-05-16-evolve-trace-shared-tree.md`
(disk-truth survives only if peer agents don't clobber it)

## How To Apply

1. **Read the ledger first.** Every cycle's first action is
   `tail -n 50 .agents/evolve/cycle-history.jsonl` (or the
   `productionLoopReader` adapter). Recover state from there, not from prior
   turns' messages.
2. **Append, don't overwrite.** Each cycle appends one JSONL line. Never
   rewrite earlier cycles. The ledger is the audit trail.
3. **Bead-driven work selection.** `bd ready` is the queue. Beads live in
   Dolt; the conversation never owns work scheduling.
4. **Falsify aspirational rules.** Any rule in `SKILL.md` that depends on
   the agent following it (vs. a harness gate enforcing it) will be
   violated within 4-6 cycles. Wire automation or accept the rule is
   guidance only.
5. **Treat "complete" claims with suspicion.** When a cycle says "all
   surfaces wired," the next CI run often finds one more. Add a per-cycle
   drift sweep instead of trusting prior claims.
6. **Protect disk-truth in shared trees.** See
   [../../swarm/references/shared-checkout-discipline.md](../../swarm/references/shared-checkout-discipline.md)
   — disk-truth only survives if peer agents don't clobber it via
   `git add -A` or destructive recovery.

## Implementing References (Already In `/evolve`)

The axiom is the principle. Each reference below is one mechanism that
keeps disk-truth working:

| Reference | Role |
|---|---|
| [cycle-history.md](cycle-history.md) | JSONL format, recovery protocol, kill switch |
| [autonomous-execution.md](autonomous-execution.md) | ScheduleWakeup self-perpetuation pattern |
| [context-budget.md](context-budget.md) | `CONTEXT_BUDGET_EXHAUSTED` as a third stop reason |
| [convergence-mechanics.md](convergence-mechanics.md) | Read-path mechanisms (prior-failure injection, healing-first classifier) |
| [scout-mode.md](scout-mode.md) | Scout-cycle as a first-class disk-truth-only result type |
| [gate-hygiene.md](gate-hygiene.md) | Pre-gate source-surface detection + structural gate-output parsing |
| [metronome-gate.md](metronome-gate.md) | Cross-cycle detector blocking same-mode-repeated failure |
| [pre-flight-schema-check.md](pre-flight-schema-check.md) | Cheap field-fit check before architectural migration |

## See Also

- `skills/rpi/references/orchestrator-compression-anti-pattern.md` — when
  the orchestrator skips phases by inlining sub-skill work, no flywheel
  artifact lands (same disk-truth principle, applied to /rpi).
- `skills/swarm/references/shared-checkout-discipline.md` — disk-truth only
  survives if peer agents don't clobber it.
