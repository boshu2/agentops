---
title: "AgentOps vs everything-claude-code (affaan-m)"
description: "Cross-harness performance system vs. context library — different optimizations."
permalink: /comparisons/agentops-vs-everything-claude-code
last_reviewed: 2026-05-07
---

# AgentOps vs. everything-claude-code

> **everything-claude-code** ([affaan-m/everything-claude-code](https://github.com/affaan-m/everything-claude-code))
> positions itself as *"the performance system for AI agent harnesses"* — DRY
> parity across Claude Code, Cursor, Codex, OpenCode, and Gemini, shipping
> **48 subagents and 182 skills** with a single set of definitions kept in sync
> across all five runtimes.

> **Note on social proof.** The everything-claude-code README claims 140K+ stars
> and 21K+ forks. The entire Claude Code plugin ecosystem is below those numbers
> as of 2026-05-07; AgentOps does not validate that claim. The comparison below
> evaluates the project on its concrete features, not the unverified social-proof
> metric.

---

## At a glance

| Aspect | everything-claude-code | AgentOps |
|---|---|---|
| **Category** | Cross-harness skill collection | Context library / wiki for agents |
| **Multi-runtime claim** | DRY parity across 5 runtimes (Claude Code, Cursor, Codex, OpenCode, Gemini) | Skills run on 4 runtimes; *and* mix-and-match models per phase within one session |
| **Inventory** | 182 skills, 48 subagents | 73 skills + your accumulated corpus |
| **Persistence** | In-session | Cross-session via `.agents/` corpus |
| **Discipline mechanism** | DRY parity tooling | Multi-model councils + RPI phase contracts |
| **Off-API surface** | None (in-session only) | `ao daemon` runs dream / evolve / compile overnight |
| **Open source** | Yes | Yes (forever) |

The two products optimize for different axes. everything-claude-code optimizes
for *"the same skill set works in every harness."* AgentOps optimizes for
*"the same model-routed workflow accumulates context across every session."*

---

## The model-independent-per-phase distinction

This is the load-bearing difference, and it is easy to miss.

everything-claude-code's "cross-harness parity" is **multi-runtime distribution
of a Claude-shaped workflow**: one set of skill and subagent definitions, kept
DRY across five harnesses, so a developer who lives in Cursor today can move to
Codex tomorrow and keep the same skill surface. That is real engineering work —
maintaining parity across five runtime conventions is harder than it sounds —
and the value is portability across editors.

AgentOps's **per-phase model routing** is a different shape entirely: inside a
single RPI loop, Claude does discovery, Codex implements, fresh Claude
validates, all in one workflow with state preserved across the model
boundaries. The handoff is the feature.

```
$ ao rpi "add rate limiting to /login"
[research/claude]    found 3 prior auth changes in .agents/decisions/
[plan/claude]        proposed: token bucket, 5/min per IP, Redis-backed
[pre-mortem/codex]   WARN: Redis unreachable case unhandled
[implement/codex]    wrote middleware/ratelimit.go, 2 tests
[validate/claude]    go test ./... PASS, gate: WARN — missing jitter
[recorded]           .agents/runs/2026-05-07-rate-limit/
```

The labels in brackets are the entire pitch made literal. Each phase picks the
model that is best for that phase; the validation phase sees a fresh context
window so it does not rubber-stamp work it just produced. Skills in any
harness — including everything-claude-code's parity-shipped catalog — inherit
the harness's single active model. They do not compose model choices per phase
within one workflow. Nobody else in the ecosystem does this.

Cross-harness parity and per-phase model routing are not the same capability.
The first is *"my skill set is portable."* The second is *"my workflow uses
multiple models inside one task."* A team can want both, but a buyer should
not assume one implies the other.

---

## When to pick which

- **Pick everything-claude-code** if your bottleneck is *"I switch between
  Claude Code, Cursor, and Codex daily and want the same skills everywhere."*
  Cross-harness parity is the headline value, and 182 skills + 48 subagents
  is a sizeable catalog out of the box.
- **Pick AgentOps** if your bottleneck is *"my agents keep re-learning the
  same lessons because nothing persists between sessions."* The corpus is
  the moat; the per-phase model routing and council validation are the
  discipline that makes the corpus trustworthy.
- **They are not mutually exclusive.** Install both: use
  everything-claude-code's catalog as portable skill inventory across
  whichever harness you happen to be in today, and use AgentOps for the
  context library that compounds across all of them.

---

## Where everything-claude-code wins

**Cross-harness DRY parity tooling.** Maintaining the same skill and
subagent definitions across five runtimes — each with its own conventions
for tool surfaces, file locations, and execution model — is real engineering
work. If you genuinely live across multiple harnesses and value the same
skill being available the same way in each one, that is a concrete win
AgentOps does not directly compete with. AgentOps ships skills for four
runtimes (Claude Code, Codex, OpenClaw, Cursor) but does not market its
parity-maintenance pipeline as the headline feature; the headline is the
corpus that the skills produce.

**Catalog size for a single-developer audience.** 182 skills + 48 subagents
is a larger pre-built catalog than AgentOps's 73 skills, and a buyer whose
question is *"how many skills can I install today"* will see that number
first.

---

## Where AgentOps wins

**Persistent corpus.** AgentOps's `.agents/` directory is a markdown wiki in
your repo, version-controlled with your code. Every session writes
learnings, decisions, citations, and validation verdicts; future sessions
read them through decay-ranked retrieval (`ao inject`). everything-claude-code
ships skills that run in-session; the work ends when the session does.

**Multi-model councils.** `/council --mixed` runs Claude and Codex judges
in parallel against one evidence packet and returns structured consensus
before commit. This is a validation primitive that catches "looks good to
one model" failure modes before the bug ships. A skill catalog — however
large or harness-portable — does not provide this surface.

**Model-independent phase routing inside one RPI loop.** Per the section
above: pick the model that is best for each phase, with state preserved
across the boundaries. This is a workflow-level capability that is
orthogonal to (and does not depend on) cross-harness parity.

**Off-API daemon on your hardware.** `ao schedule` and `ao daemon` run
dream / evolve / compile / defrag / forge passes against your subscription,
off-vendor, overnight. The corpus compounds while you sleep.
everything-claude-code is in-session by design; the off-API surface is not
part of its category.

---

## Both are open source

Both projects are open source. AgentOps is open source forever — the
corpus that compounds in *your* repo is yours, the schema is portable,
and the discipline survives any single vendor's roadmap. If you are
evaluating durability of either project, the source code is the receipt;
inflated star counts and other unverified metrics are not.

---

## Bottom line

Different optimization targets, not direct competition.

everything-claude-code is selling **cross-harness portability**: one skill
set, five runtimes, kept in DRY parity by the maintainers' tooling. If
your friction is editor switching, that is real value.

AgentOps is selling **corpus discipline + per-phase model routing**: the
operational layer that turns each session's research, decisions,
validations, and learnings into a markdown wiki your agents read on the
next session, and the workflow primitive that lets you compose Claude,
Codex, and other models per phase inside one task.

A buyer who lives across harnesses and does not yet care about persistence
should pick everything-claude-code. A buyer who is bottlenecked on
"my agents forget what we learned last week" should pick AgentOps. A
buyer with both bottlenecks should install both — they sit at different
layers of the stack.

---

<div align="center">

[← Back to Comparisons](README.md)

</div>
