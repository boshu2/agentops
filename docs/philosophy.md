---
last_reviewed: 2026-04-12
---

# AgentOps Philosophy

## The Problem

Coding agents are good at thinking. They are bad at bookkeeping.

Every session starts cold. The agent that spent two hours debugging a timeout bug last Tuesday has no memory of it. The pattern you hard-won in session 3 is gone by session 15. The planning rule that would have prevented a regression sits buried in a transcript no one reads.

This is not a model problem. It is an environment problem. The model is capable. The environment around it does not compound.

## What AgentOps Is

AgentOps is a context compiler.

The compiler analogy is exact: raw session signal (decisions, failures, patterns, warnings) is processed through extraction, scoring, curation, and promotion into reusable artifacts — learnings, findings, planning rules, enforcement gates. The next session runs against a richer environment than the last. The model stays the same. The environment gets smarter.

This maps directly to what Andrej Karpathy observed about knowledge work: the tedious part is not the thinking, it is the bookkeeping. Organizing, surfacing, routing, and keeping knowledge fresh. AgentOps automates that layer.

## The Flywheel

```
Sessions → Bookkeeping → Learnings → Findings → Planning Rules → Gates
    ↑                                                                 │
    └─────────────────────── Better next session ─────────────────────┘
```

Each phase is deliberate:

- **Sessions** produce signal: commits, decisions, failures, retros.
- **Bookkeeping** (`/retro`, `/forge`, `ao harvest`) extracts and scores that signal. Scores on specificity, actionability, novelty, and confidence filter noise.
- **Learnings** are the raw output — scored, attributed, timestamped.
- **Findings** are promoted learnings: higher confidence, cross-session validation, broader applicability.
- **Planning rules** are enforcement-level knowledge: if a finding is violated, the pre-mortem blocks the plan.
- **Gates** are automated checks in `/pre-mortem`, `/vibe`, and `/council` that prevent known failure modes before they ship.

The loop closes. The system does not just capture knowledge — it enforces it.

## The Data Format

`.agents/` is the universal data format.

Plain markdown files, versioned in git, readable by any LLM, browsable in Obsidian, diffable in any editor. No embeddings, no vector database, no proprietary store.

This is a deliberate bet against the current tooling consensus. Vector databases optimize for semantic recall at scale. `.agents/` optimizes for editorial control, freshness management, and human legibility. For a codebase knowledge base where:

- Volume is bounded (one project, not the internet)
- Freshness matters more than recall breadth (stale knowledge is worse than no knowledge)
- Human curation is the highest-leverage action
- Portability is required (no cloud dependency, works air-gapped)

...markdown + wikilinks outperforms embeddings. The agent can grep it, the human can read it, and `ao defrag` can maintain it.

## The Tiered Model

Not every knowledge operation needs a frontier model. AgentOps uses three tiers:

| Tier | When | Why |
|------|------|-----|
| Local 8B (ollama, etc.) | Volume work — dedup, defrag, freshness scoring, overnight compounding | Fast, private, cheap. Runs while you sleep. |
| Frontier (Claude, GPT-4o, etc.) | Quality work — council validation, pre-mortem review, pattern extraction | Accuracy matters more than throughput. |
| Human | Curation and promotion decisions | Judgment calls that agents get wrong systematically. |

`/dream` and `ao overnight` use the local tier for continuous compounding. `/council` and `/pre-mortem` use the frontier tier for high-stakes validation. The human reviews promotions from learning → finding → rule.

The ratio is intentional. Validation and curation cost 3-5x implementation time. This is not overhead — it is the ratchet. Without it, the flywheel runs backward.

## The Ratchet

AgentOps adopts the Brownian Ratchet as a first principle: embrace agent variance, filter aggressively, and make progress one-way.

Agents produce noisy output. Some sessions are brilliant; some are catastrophic. The naive response is to constrain the agent. The AgentOps response is to ratchet: let variance happen, filter at gates (`/pre-mortem` blocks bad plans, `/vibe` blocks bad code, `/council` blocks bad decisions), and only let good output advance. The gate is asymmetric — easy to pass in the forward direction, impossible to pass backward.

This is why validation gates are blocking, not advisory. An advisory gate with no enforcement is not a ratchet. It is a suggestion.

## What This Is Not

AgentOps is not a chatbot wrapper. It does not make prompts bigger. It does not add more agents to the same problem.

It is not trying to replace thinking. The model thinks. AgentOps manages what the model knows when it thinks.

It is not a SaaS product or a managed service. All state lives locally. All operations are reversible. The product is the compounding environment — the `skills/`, the `ao` CLI, and the discipline enforced by the hooks. That environment is yours to own, version-control, and take with you.

## The Validated Thesis

As of April 2026, the flywheel thesis is empirically confirmed on a single production repo (this one):

- 163 learnings extracted, scored, and curated
- 13 planning rules enforced at pre-mortem gates
- 12 patterns promoted from repeated findings
- 10/12 `ao doctor` checks passing with full 7/7 hook coverage

The compound growth is measurable. Session 1 started cold. Session 100+ starts with a knowledge corpus that catches known failure modes before implementation begins.

That is the point. Not a bigger prompt. A repo that remembers.
