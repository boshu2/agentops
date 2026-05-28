---
title: /loop context accumulates across cron fires — 87-cycle empirical observation
date: 2026-05-13
tags: [loop, cron, context-budget, operational-observation]
source: cycles 44-131 of the 2026-05-12 → 2026-05-13 /evolve session
companion: soc-wx55q.1 (test design — already filed)
---

# /loop context drift — empirical observation

soc-wx55q.1 was filed 2026-05-09 based on a 9-fire real-loop test
that observed ~16-24K tokens of overhead accumulation. This
learning records a 87+ fire observation from cycles 44-131 of the
ongoing 2026-05-12 → 2026-05-13 /evolve session.

## Observed behavior

The current /loop implementation (`/loop 1 min /evolve`) re-fires
the same parent prompt every minute. Each fire arrives with:

- The full skill prompt repeated verbatim (~5K tokens per fire of
  the /evolve skill text alone)
- A `<system-reminder>` injection (varies — could be the bd ready
  reminder, the task-tool reminder, hook-injection notes)
- The prior turn's tool results retained in conversation history
  (cycle history, git status, code diffs)

After ~80 cycles, the parent conversation contained:
- ~150+ skill-prompt re-injections (visible from the conversation
  log's structure)
- Repeated `<command-message>agentops:evolve` markers
- Cumulative tool-result history from every cycle (commits, bd
  outputs, build logs)

The harness reports "automatic compression" handles this so the
context window isn't blown — and indeed the session ran 87+
productive cycles without manual intervention. So the practical
threat from soc-wx55q.1 (auto-compact dropping critical state) did
NOT materialize in this session.

## What DID happen

Two real but soft effects accumulated:

1. **Re-discovery overhead.** Several cycles re-derived information
   that was established in earlier cycles. Examples:
   - Cycle 125 re-discovered the Session-is-3-concepts split
     (originally surfaced in the cycle-51 bounded-context research).
   - Cycle 126 re-discovered the substring-overreach risk that the
     audit-then-execute discipline (cycle 125) was meant to prevent.
   - Cycle 128 re-discovered that drift catalogs over-flag (a
     pattern that was implicit in the cycle 122 learning but not
     explicit until cycle 129).
   - In each case, the cycle-N work referenced a prior cycle's
     conclusion but had to re-do enough of the work to confirm it.
     Net: the same finding is now captured in 2-3 places.

2. **Hook-noise injection.** Multiple cycles received PreToolUse
   hook reminders that didn't fire on every cycle but did fire
   sporadically. The hook injections crowd the conversation
   without changing behavior (the same Go standards reminder
   appears 30+ times across the session). Practical effect:
   minor — the reminders are short and pattern-matched on irrelevance.

## What did NOT happen (relative to soc-wx55q.1 fears)

- **No auto-compact dropping critical state.** The cycle-history
  ledger (`.agents/evolve/cycle-history.jsonl`) is durable and
  read at every cycle start, so even if conversation context were
  dropped, the ledger preserves the work record. This is the
  intended design.
- **No /loop fire failed to do useful work due to context loss.**
  Every cycle that intended to act produced output.
- **No "9 fires of pure no-op overhead" pattern recurred.** Each
  cycle in this session had a real signal source (harvested item,
  bd ready bead, generator output, audit finding, contract drift,
  knowledge-capture opportunity).

## Why this session ran cleanly despite soc-wx55q.1's concern

Three structural decisions made the difference:

1. **Cycle-history ledger as the source of truth.** Every cycle
   starts by recovering state from
   `.agents/evolve/cycle-history.jsonl` — not from the
   conversation. Context drift in the conversation doesn't lose
   the work-state.

2. **Bookkeeping is local-write, not conversation-state.** Cycle
   bookkeeping appends to the ledger; the LLM doesn't need to
   re-summarize the previous cycle.

3. **`bd` tracks queueable work out-of-band.** Beads live in Dolt,
   not in the conversation. `bd ready` produces fresh signal at
   every cycle.

## Test design recommendation for soc-wx55q.1

Direct empirical observation suggests:

- The "/clear in prompt + cold start per fire" option (#1 in the
  original bead) is **already effectively how this session
  worked** — cycle-history.jsonl is the cold-start substrate. The
  conversation context that accumulates is mostly decorative; the
  load-bearing state is on disk.
- The "long-lived factory tmux" concern was about state being LOST
  if /loop fires don't share context. In practice, the disk-based
  state survives, so loss-on-context-drop isn't the problem.
- The actual cost is **token spend on harness overhead**, which is
  real but not load-bearing on correctness. If the user accepts
  the token cost, /loop works as a daemon replacement.

## Where this observation should land

soc-wx55q.1 is filed as P1 must-fix; this learning suggests it
might be P3 (worth doing but not blocking 3.0 release). The
"daemon replacement" framing in the cut sheet survives because
correctness doesn't depend on conversation context — durable
state on disk is what matters.

## See also

- `docs/learnings/2026-05-13-bc-ports-wire-up-arc.md` (cycle 122)
  — captures another 11-cycle arc.
- `docs/learnings/2026-05-13-substring-sed-rename-overreach.md`
  (cycle 127) — substring-audit checklist that prevented context-
  cost waste on rename-overreach corrections.
- `soc-wx55q.1` — the bead this observation supports.
