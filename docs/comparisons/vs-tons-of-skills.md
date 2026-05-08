---
title: "AgentOps vs Tons-of-Skills (jeremylongshore/claude-code-plugins-plus-skills)"
description: "Volume marketplace vs. context library — different categories, different buyers."
permalink: /comparisons/agentops-vs-tons-of-skills
last_reviewed: 2026-05-07
---

# AgentOps vs. Tons-of-Skills

> **Tons-of-Skills** ([jeremylongshore/claude-code-plugins-plus-skills](https://github.com/jeremylongshore/claude-code-plugins-plus-skills))
> is a volume marketplace: **425 plugins, 2,810 skills, 200 agents**, a CCPI
> package manager, and a 100-point validation grading rubric. It is a different
> category from AgentOps. This page exists to clarify the comparison for buyers
> shopping both.

---

## At a glance

| Aspect | Tons-of-Skills | AgentOps |
|---|---|---|
| **Category** | Skills marketplace | Context library / wiki for agents |
| **Inventory claim** | 2,810 skills, 425 plugins, 200 agents | 73 skills + your accumulated corpus |
| **Buyer pitch** | "We have the most stuff" | "You build the moat" |
| **Validation** | 100-point CCPI grading rubric | Multi-model councils + validation gates |
| **Distribution** | CCPI package manager | Native marketplace per runtime + `curl \| bash` install |
| **Persistence** | Per-skill, in-session | Cross-session via `.agents/` corpus |
| **Open source** | Yes | Yes (forever) |

The two products answer different questions. Tons-of-Skills answers *"what skills can I install?"* AgentOps answers *"how do I make my agents remember what we already learned?"*

---

## When to pick which

- **Pick Tons-of-Skills** if you want a bulk-buy of pre-built skills covering many domains, with a package manager and a published grading rubric to filter by.
- **Pick AgentOps** if you want to build a context library that compounds across sessions — a wiki for your agents, persisted in your repo, that grows with every run.
- **They are not mutually exclusive.** Install both. Use Tons-of-Skills as a skill catalog for breadth; use AgentOps for the corpus discipline that turns one-off skill outputs into accumulated knowledge.

This is the cleanest framing: a marketplace and a context library are complementary tools. The marketplace gets you skills on day one. The context library is what makes day 100 different from day 1.

---

## Where Tons-of-Skills wins

**Inventory breadth.** 2,810 skills is the largest catalog in the Claude Code ecosystem. If your bottleneck is "I need a skill for X and don't want to write one," Tons-of-Skills has the highest probability of already having it. AgentOps ships 73 skills focused on the operational layer (research, plan, validate, harvest, council, etc.) — the inventories don't overlap heavily.

**Validation grading rigor.** The 100-point CCPI grading rubric is a public scoring system applied per-skill. That is a real artifact: a published, comparable score across thousands of skills. AgentOps does not grade skills on a 100-point scale; its quality posture is multi-model council consensus and validation gates run *on the agent's output*, not on the skill definition itself. Different surface, different rigor.

**Package management.** CCPI is a dedicated package manager for skills. AgentOps installs via the Claude Code, Codex, OpenClaw, and Cursor native marketplaces plus a `curl | bash` script — there is no AgentOps-specific package manager because AgentOps treats the runtime's marketplace as the distribution layer.

---

<!-- agentops:claim:AOP-CLAIM-COMP-TONS-OF-SKILLS-CORPUS-COMPOUNDING -->

## Where AgentOps wins

**Persistent corpus.** AgentOps's `.agents/` directory is a markdown wiki in your repo, version-controlled with your code. Learnings, decisions, citations, and validation verdicts are extracted and stored every session, then injected into future sessions via decay-ranked retrieval. Tons-of-Skills ships skills; the skill output ends with the session. AgentOps ships a bookkeeping schema that turns each session's output into durable corpus that the next session reads.

**Multi-model councils.** `/council --mixed` runs Claude and Codex judges in parallel against one evidence packet, returning structured consensus before commit. This is a validation primitive, not a skill — it is the thing that catches "looks good to one model" before the bug ships. Tons-of-Skills's grading rubric scores skills before you install them; AgentOps's councils score *the work the agent just did* before it lands.

**Model-independent phase routing.** Inside a single RPI loop, AgentOps runs Claude for research, Codex for implementation, fresh Claude for validation, with state preserved across the boundaries. Skills in any marketplace inherit the harness's model — they don't compose model choices per phase. This is a workflow-level capability, orthogonal to skill inventory.

**Off-API daemon.** `ao schedule` and `ao daemon` run dream / evolve / compile / defrag / forge passes on your hardware against your subscription, off-vendor, overnight. The corpus compounds while you sleep. Tons-of-Skills is in-session by design; the off-API surface is not part of its category.

---

## Both are open source

Both projects are open source and intended to stay that way. Tons-of-Skills is MIT-licensed on GitHub with sponsorship-funded development. AgentOps is open source forever — the corpus that compounds in *your* repo is yours, the schema is portable, and the discipline survives any single vendor's roadmap. If you are evaluating durability of either, the source code is the receipt.

This matters more than it sounds. The structural risk in the agent ecosystem is "vendor ships Managed Agents and eats the plugin." Open-source skill inventories and open-source context libraries both survive that move; they live in your repo, not in someone's hosted product.

---

## Bottom line

These are different categories, not competition.

Tons-of-Skills is selling **inventory**: the largest catalog of pre-built skills in the ecosystem, with a package manager and a published grading rubric. If you measure by "skills installed per developer," it wins by construction.

AgentOps is selling **corpus discipline**: the operational layer that turns each session's research, decisions, validations, and learnings into a markdown wiki your agents read on the next session. If you measure by "what the agent knows about this codebase that it didn't know last week," AgentOps is the substrate that makes that question answerable.

A buyer shopping both should install both. Use Tons-of-Skills when the question is *"is there a skill for this?"* Use AgentOps when the question is *"how do we stop re-learning the same lessons every session?"*

---

<div align="center">

[← Back to Comparisons](README.md)

</div>
