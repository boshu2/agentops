# The Knowledge Flywheel

> Agents are stateless. The repo learns.

## The Problem

Coding agents forget everything between sessions. Notes alone do not fix that. If a solved problem is not extracted, curated, retrieved, and reused, the repo keeps paying for the same lesson.

AgentOps frames this as two of the three gaps in the [Context Lifecycle Contract](context-lifecycle.md):

- **Durable learning** (Gap 2) — solved problems recur because knowledge is not extracted, scored, and surfaced.
- **Loop closure** (Gap 3) — completed work does not produce better next work because learnings are not harvested, promoted, or fed back into future sessions.

The flywheel is the mechanism that closes both gaps. Each stage below maps to one or both.

## The Solution

AgentOps turns session output into durable environment state. The automation path depends on the runtime: hook-capable runtimes can drive startup and closeout automatically, while Codex uses explicit lifecycle commands that provide the same flywheel stages without pretending hooks exist.

## Runtime Modes

| Mode | Start path | Closeout path | What is automatic |
|------|------------|---------------|-------------------|
| Hook-capable runtime | SessionStart hook or `ao inject` | SessionEnd/Stop hooks or `ao forge transcript` + `ao flywheel close-loop` | Startup retrieval, transcript forging, pool maintenance when hooks are installed |
| Codex hookless fallback | `ao codex start` | `ao codex stop` | Startup context assembly, transcript discovery fallback, citation capture, and close-loop status through explicit commands |
| Manual fallback | `ao inject` / `ao lookup` | `ao forge transcript` + `ao flywheel close-loop` | Nothing hidden; operator runs the lifecycle directly |

## The Flywheel

```
┌───────────────────────────────────────────────────────────────────────┐
│                     THE KNOWLEDGE FLYWHEEL                            │
│                                                                       │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐       │
│  │  1. WORK   │─>│  2. FORGE  │─>│  3. POOL   │─>│ 4. PROMOTE │       │
│  │  Session   │  │  Extract   │  │  Score &   │  │  Graduate  │       │
│  │            │  │            │  │  Queue     │  │            │       │
│  └────────────┘  └────────────┘  └────────────┘  └────────────┘       │
│       ^                                                  │            │
│       │         ┌────────────┐  ┌────────────┐           │            │
│       └─────────│  6. INJECT │<─│5. LEARNINGS│<──────────┘            │
│                 │  Surface   │  │  Permanent │                        │
│                 │  & Cite    │  │  Knowledge │                        │
│                 └────────────┘  └────────────┘                        │
│                                                                       │
│  Each citation feeds back: utility scores update, high-utility        │
│  knowledge surfaces more often, low-utility decays. This is the       │
│  compounding effect — sessions get smarter because the best           │
│  knowledge rises and the noise sinks.                                 │
└───────────────────────────────────────────────────────────────────────┘
```

## The Six Stages

Each stage maps to the gaps it closes: **L** = Durable Learning (Gap 2), **C** = Loop Closure (Gap 3).

### Stage 1: Work (source material)

You build, debug, research, or plan. In hook-capable runtimes, transcripts are typically available directly from the runtime. In Codex, AgentOps prefers archived session transcripts and can fall back to `~/.codex/history.jsonl` when no archived transcript exists.

### Stage 2: Forge — **L** (extraction)

At closeout, `ao forge transcript` or `ao codex stop` parses the transcript and extracts structured knowledge — decisions, solutions, learnings, failures, and references. Each becomes a markdown file in `.agents/knowledge/pending/`. In hook-capable runtimes, the `SessionEnd` hook (`session-end-maintenance.sh`) triggers this automatically; the `athena-session-defrag.sh` hook runs deduplication and defrag in the same event.

### Stage 3: Pool — **L** (curation)

`ao flywheel close-loop` ingests pending files and scores each on five dimensions:

| Dimension | What it measures |
|-----------|-----------------|
| Specificity | Names concrete files, functions, error messages |
| Actionability | A future session can act on this without more context |
| Novelty | New knowledge, not repetition |
| Context | Explains WHY, not just WHAT |
| Confidence | How certain the extraction is |

Candidates are tiered: **Gold** (>0.85), **Silver** (0.70–0.85), **Bronze** (0.50–0.70), or **Discard** (<0.50).

### Stage 4: Promote — **L** + **C** (graduation)

Candidates that pass the promotion gate graduate to permanent knowledge:

- **Age gate:** Must be >24h old (prevents promoting noise from the current session)
- **Citation gate:** Must have been cited at least once (proves another session found it useful)
- **Tier gate:** Gold and Silver auto-promote. Bronze requires 3+ citations.

This is where durable learning and loop closure intersect: only knowledge that a later session actually cited gets promoted, proving the loop closed at least once.

### Stage 5: Learnings — **L** (permanent store)

Promoted knowledge lives in `.agents/learnings/` and `.agents/patterns/`. The maturity lifecycle:

```
provisional → established → archived
```

AgentOps maturity controls (`ao maturity --expire`, `ao maturity --evict`, `ao dedup`, `ao contradict`) prevent the corpus from decaying into stale noise. Maintenance runs through the active lifecycle path: hook-capable runtimes run it from hooks, while Codex runs the same hygiene from `ao codex start` / `ao codex stop`.

### Stage 6: Inject — **C** (retrieval closes the loop)

At session start and during work, `ao inject`, `ao lookup`, or `ao codex start`
retrieves the most relevant learnings for the current task. Startup retrieval
prefers task-scoped context such as handoff goals and active beads instead of
generic commit-subject fallbacks. `ao lookup` records citations automatically.
When `ao search` results are actually adopted, use `ao search --cite
retrieved|reference|applied` to record that decision in-band instead of relying
on tribal workflow knowledge. Each citation is the signal that drives the
feedback loop.

Citations with positive feedback increase the learning's utility score → higher utility → ranked higher in next injection → cited more → utility increases more → **compounding**. This is the loop closure mechanism: completed work produces better next work because the flywheel feeds validated knowledge back into future sessions.

## The Compounding Math

The flywheel equation:

```
dK/dt = I(t) - δ·K + σ·ρ·K
```

- **σ** — Retrieval coverage: unique surfaced artifacts / total retrievable artifacts, scale 0.0–1.0
- **ρ** — Decision influence rate: unique surfaced artifacts later evidenced by `reference` or `applied` citations / surfaced artifacts, scale 0.0–1.0
- **δ** — Knowledge age: average age of active learnings in days. The theoretical decay rate (0.17/week from Darr 1995) motivates the metric, but the CLI implementation (`metrics_health.go`) measures delta as days, not a weekly rate.
- **Escape velocity:** When `σ × ρ > δ/100`, knowledge compounds faster than it ages out. The `/100` normalizes delta (days) to a ratio comparable with sigma and rho.

### Golden Signals

Escape velocity is necessary, but not sufficient. Four golden signals measure
whether the flywheel is actually compounding:

```bash
ao flywheel status
```

| Signal | Question | Healthy |
|--------|----------|---------|
| Velocity Trend | Is σρ-δ improving over time? | Positive slope |
| Citation Pipeline | Are citations delivering value? | >60% high-utility |
| Research Closure | Is research being mined into learnings? | <10% orphaned |
| Reuse Concentration | Is the whole pool active or just a few items? | Gini < 0.4 |

## Knowledge Stores

| Store | Content | Updated By |
|------|---------|------------|
| `.agents/knowledge/pending/` | Forge output awaiting pool ingestion | `ao forge`, `ao codex stop` |
| `.agents/knowledge/pending/.quarantine/` | Low-quality or unsafe pending extracts held out of promotion | Pool hygiene, promotion gates, and close-loop maintenance |
| `.agents/pool/` | Scored candidates awaiting promotion | `ao flywheel close-loop` |
| `.agents/learnings/` | Promoted, permanent knowledge | Pool promotion pipeline |
| `.agents/patterns/` | Promoted decision patterns | Pool promotion pipeline |
| `.agents/research/` | Scoped investigations | `/research` |
| `.agents/findings/registry.jsonl` | Reusable findings | `/pre-mortem`, `/vibe`, `/post-mortem` |
| `.agents/ao/citations.jsonl` | Citation trail | `ao inject`, `ao lookup`, `ao search --cite`, `ao codex start` |
| `.agents/ao/feedback.jsonl` | Utility feedback | `ao flywheel close-loop` |
| `.agents/ao/metrics/` | Baseline snapshots for trend tracking | `ao metrics baseline` |
| `.agents/ao/codex/startup-context.md` | Explicit startup context assembled for hookless Codex sessions | `ao codex start` |
| `.agents/ao/codex/state.json` | Last Codex start/stop lifecycle state | `ao codex start`, `ao codex stop` |

## The Compounding Effect

| Gap | Without the flywheel | With AgentOps flywheel |
|-----|----------------------|------------------------|
| Durable learning | The same bug is rediscovered each session | `ao lookup` retrieves the prior failure before planning starts |
| Durable learning | Notes accumulate without pressure | AgentOps promotion gates ensure only cited, high-quality knowledge survives |
| Durable learning | Stale knowledge pollutes retrieval | `ao maturity`, `ao dedup`, and `ao contradict` keep the corpus current |
| Loop closure | Handoffs rely on chat memory | AgentOps stores handoffs and phased state on disk in `.agents/` |
| Loop closure | Session 50 starts from scratch | Session 50 starts with 50 sessions of flywheel-promoted wisdom |
| Loop closure | Completed work teaches nothing | `/post-mortem` + finding compiler + `ao-flywheel-close.sh` harvest and compile learnings automatically |

## See Also

- [Context Lifecycle Contract](context-lifecycle.md)
- [How It Works](how-it-works.md)
- [Codex Hookless Lifecycle](architecture/codex-hookless-lifecycle.md)
- [Primitive Chains](architecture/primitive-chains.md)
- [Brownian Ratchet](brownian-ratchet.md)
- [The Science](the-science.md)
