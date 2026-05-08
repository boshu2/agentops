---
title: "AgentOps as a Trust Factory"
description: "How AgentOps maps to the five-step preparation engine: identity, reproducibility, evaluation, evidence, recovery."
permalink: /trust-factory
last_reviewed: 2026-05-07
---

# AgentOps as a Trust Factory

> Generation is becoming cheap. Validation is becoming the moat. Software
> engineering has spent 50 years building trust factories — the systems that
> turn changed artifacts into trusted capability. AgentOps is that pattern
> applied to the artifacts AI coding agents produce.

A trust factory is the discipline that sits between *something changed* and
*we believe it*. Compilers, build systems, code review, CI, staging, canaries,
post-mortems — none of them generate value by themselves. They generate
*permission to ship*. As coding agents commoditize generation, the part of the
loop that earns permission is what compounds.

## The five-step primitive

Every artifact promotion needs five things, in order:

1. **Identity** — what changed?
2. **Reproducibility** — can we replay it?
3. **Evaluation** — what did it pass?
4. **Evidence** — where is the proof?
5. **Recovery** — what if we were wrong?

Drop any one of them and trust collapses. Identity without reproducibility is
a name with no body. Evaluation without evidence is a claim no one can audit.
Evidence without recovery is a museum. The five together are what turn a code
change, a model weight, a config, or a mission plan into something an
organization is willing to stand behind.

## How AgentOps maps to the primitive

| Trust factory step | AgentOps mechanism |
|---|---|
| Identity | `.agents/runs/<id>/` packets, citations, versioned context |
| Reproducibility | RPI phase contracts + worktree isolation + captured packets |
| Evaluation | `/pre-mortem`, `/vibe`, `/council`, `ao goals measure` |
| Evidence | Council verdicts, citations, ratchet records, post-mortems |
| Recovery | Ratchet rollback, learnings → planning rules → prevention |

**Identity.** Every agent run gets a discovery packet under `.agents/runs/<id>/`
that records what was being changed, which corpus entries were injected, and
which sources were cited. The `ao inject` retrieval that loaded the working
context is logged so a later reviewer can ask *what did the agent actually
know when it acted?* without guessing.

**Reproducibility.** The RPI workflow (Research → Plan → Implement → Validate)
runs each phase against a written contract, and `ao crank` isolates parallel
work in worktrees so the inputs to a phase can be replayed. The packet captured
at planning time is the same packet a re-run consumes; deviations show up as
diffs against a known input, not as drift.

**Evaluation.** Plans are stress-tested with `/pre-mortem` before code is
written, code is challenged with `/vibe` before it leaves the working tree, and
high-stakes changes go through `/council` for multi-judge consensus. Strategic
fitness is checked separately with `ao goals measure`, which asserts that the
change actually moves the GOALS.md needles instead of merely passing tests.

**Evidence.** Council verdicts, citation logs, ratchet records, and the
findings written by `/post-mortem` are durable artifacts under `.agents/`. They
stay diffable in git, so the proof of why a change was promoted lives next to
the change itself, not in a SaaS tool that the next vendor can take away.

**Recovery.** When a change turns out to have been wrong, the ratchet record
is the rollback unit, and the failure is converted by `/post-mortem` and
`/forge` into a learning, then into a planning rule that prevents the same
class of mistake from being injected again. The loop is closed inside the
corpus, not inside a chat transcript.

## Why this primitive generalizes

The same five steps apply to anything that gets promoted: model weights,
mission plans, agent actions, infrastructure configs, training data. Identity,
reproducibility, evaluation, evidence, and recovery are how regulated
industries already think about change — they just call it configuration
management, model risk management, or operational acceptance depending on the
substrate.

AgentOps is not the trust factory for all of those substrates. It is the first
instance of the pattern *for the artifacts AI coding agents produce*: code
diffs, learnings, planning rules, skill changes, corpus updates. The pattern
travels; the implementation is scoped. As other artifact classes get their own
agent-shaped pipelines, the same five-step primitive is what each will need.

## When to use this framing vs. the wiki framing

The [wiki framing](./wiki-for-agents.md) is the right opener for the busy
engineer who responds to deflationary framing — "it's a wiki for your agents,
version-controlled in your repo." It earns trust by under-claiming.

The trust-factory framing is the right opener for engineering leads in
regulated environments who already think in terms of identity, reproducibility,
evaluation, evidence, and recovery. They recognize the vocabulary and skip the
explanation tax. Lead with this framing for buyers who already know that
"validation is the moat"; lead with the wiki framing for buyers who want to
see the artifact before the philosophy.

Both framings describe the same product. They optimize for different listeners.

## See also

- [README](https://github.com/boshu2/agentops/blob/main/README.md) — product-level framing and dogfood receipts
- [PRODUCT.md](https://github.com/boshu2/agentops/blob/main/PRODUCT.md) — internal positioning and four-layer model
- [docs/wiki-for-agents.md](./wiki-for-agents.md) — the wiki-framing companion
- [docs/cdlc.md](./cdlc.md) — the Context Development Lifecycle in full
- [docs/the-science.md](./the-science.md) — knowledge-decay and compounding model
