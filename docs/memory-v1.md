# Memory v1 — the contract (what we remember, and how ao serves it)

> Status: DRAFT for Bo's ratification (2026-06-28). Forcing function demanded by the
> agile-founder council — the decision avoided for 8 months. Retrieval was never the
> blocker; **deciding what to remember** was. Ratify or edit this, then ship.

## The decision (SOT)

**`ao` is the unified agent memory.** Files under `.agents/` (per-repo) and `~/.agents/`
(per-machine hub) are the source of truth; `ao` is the access path. Not a DB, not a graph,
not a platform. `ao` is already harness-agnostic — Claude, Codex, and Gemini all call it —
so making it the front door *is* "one memory all my agents share."

`cm` and the 13 Claude per-project silos become **inputs to ingest**, then retire on a date.
One SOT.

## What we remember (narrow — the moat is curation, not storage)

Durable operational facts that **change future agent behavior**:
- **Decisions** + the why (architecture forks, one-way doors, rulings).
- **Durable constraints / rules** (LAW 0, scope boundaries, invariants).
- **Preferences** (how Bo wants work done — confirmed feedback).
- **Failure→rule promotions** (a mistake that recurred twice becomes a remembered rule).
- **Project state not derivable from code or git** (intent, goals, open threads).

## What we do NOT remember (delete-on-sight)

Summaries, logs, "vibes," speculative knowledge, transient task state, anything derivable
from the repo/git, raw transcripts. *Stale memory degrades output more than no memory.*

## When it's written (curated, not auto-captured)

- On explicit `/remember` (human/agent-gated).
- On a decision made or a failure-repeated-twice → promoted to a rule.
- NEVER auto-ingest everything (that's the cognee/graphify trap and the anti-signal Bo's
  whole instinct rejects).

## Who reads it / when

- At **session start** — inject top relevant hits (harness inject hook).
- On demand — `ao recall "<query>"` from any agent.

## Retirement / decay

ao's freshness decay (`exp(-ageWeeks·0.17)`) + maturity weighting already handle aging.
A fact superseded by a newer fact on the same key is last-write-wins. Explicit retirement
via the normal artifact lifecycle. No temporal graph needed for v1.

## Done criteria (v1 — declare done when ALL hold)

1. `ao recall "<query>"` returns top-N **cited** facts (source path + tier), over the
   EXISTING lexical + decay + maturity + MemRL scorer. **No dense retrieval. No CGO.**
2. The 13 Claude silos are ingested as curated markdown with provenance (id, timestamp,
   source path, title) — content preserved, not LLM-"extracted."
3. A **25-query acceptance set** of real questions Bo expects agents to answer; ship gate =
   **top-5 contains the right cited memory for ≥20/25.**
4. Wired into ≥1 real AgentOps workflow (session-start inject on at least one harness) and
   used on a real task without re-deriving a known fact.

## Non-goals (NEVER build — hold the line)

Knowledge graphs · LLM-extraction ingestion pipelines · vector DBs · contradiction/temporal
engines · multi-hop reasoning · ontologies · sync daemons · memory UIs · auto-curation · a
4th memory store · any Python in the memory stack.

**Dense/semantic retrieval is DEFERRED, not rejected** — it's the correct v2, but it is
*earned only by a miss-log*: real `ao recall` usage showing lexical misses that cost value.
Until that log exists, adding onnx/CGO to a pure-Go binary is over-engineering.

## Why this is the best-science answer (not a cop-out)

Mem0 deleted its graph layer; Letta showed plain files beat bespoke memory APIs (74.0 vs
68.5); the shipping SOTA is hybrid retrieval + curated facts + decay, with graphs reserved
for a *named* requirement. ao already implements the curated-facts + decay + (lexical) half.
v1 ships that. Dense is the documented last-2-points upgrade, gated on evidence.
