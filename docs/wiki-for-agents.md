---
title: "A wiki for your agents"
description: ".agents/ is just markdown in your repo, version-controlled with your code, that agents read and contribute to."
permalink: /wiki-for-agents
last_reviewed: 2026-05-07
---

# A wiki for your agents

> Wikis aren't new. They've been around for 25 years. The novel part is building one for agents to read, traverse, and contribute to — and making the discipline of maintenance mechanical so it actually happens.

Every engineer has used a wiki. MediaWiki, Confluence, Notion, a `docs/` directory that grew teeth. AgentOps is that shape, with two changes: the readers are agents, and the writers are agents too.

## What `.agents/` actually is

Run `ls .agents/` in an initialized repo. Plain directories of plain markdown:

```
.agents/
├── council/         # multi-judge verdicts on plans, PRs, releases
├── decisions/       # what was chosen, what was rejected, why
├── findings/        # surprises and gotchas surfaced by sessions
├── learnings/       # promoted lessons that survived review
├── patterns/        # recurring shapes worth reusing
├── planning-rules/  # constraints that fire during planning
└── runs/            # per-session packets with citations and evidence
```

`tree .agents/ -L 1` shows the rest. Every leaf is a markdown file. The whole tree is committed alongside your code: version-controlled, diffable, branchable, mergeable. Nothing about it is exotic. That is the point.

## Why most wikis fail

Writing the wiki is a tax humans pay grudgingly, after the work is already done. The tax falls hardest on the contributors who learned the most — the ones whose attention is most valuable elsewhere.

So the wiki bitrots. Pages drift behind the code. The "current architecture" page describes a system that was rewritten last quarter. New hires read it once, find it lying, and stop trusting it.

By week four it is a read-only artifact. Search breaks because no one tags. The team falls back to Slack scrollback and the one engineer who remembers everything.

AgentOps inverts this. Sessions write to the wiki by default — runs land citations, councils land verdicts, post-mortems land learnings, the daemon defrags overnight. The agents that consume `.agents/` also produce it. Maintenance is mechanical, not voluntary. The wiki maintains itself because it sits on the path of everything else.

## Why not Notion or Confluence?

| Notion / Confluence | AgentOps `.agents/` |
|---|---|
| Written for humans; agents can't traverse it efficiently | LLM Wiki of Markdown — agents read it natively |
| Lives in SaaS, not your repo | Lives in `.agents/` next to the code |
| Not version-controlled with your code | Diffable, branchable, mergeable |
| No decay ranking, no retrieval scoring | `ao inject` returns decay-ranked, token-budgeted packets |
| No validation gates, no automated capture | Sessions write to it automatically; councils validate it |
| Doesn't compound; you maintain it manually | Daemon defrags, evolves, and compounds it overnight |
| Read-only artifact | Writes itself: agents that use it also produce it |

**Native agent traversal.** Markdown is the format models read best; a SaaS page has to be exported and re-embedded before an agent can use it.

**Locality.** "What changed in this PR" includes the wiki delta. Reviewers see context move with the code instead of switching tabs.

**Version control.** Diffs, branches, and merges are how engineers already think about change. A wiki you can `git revert` is a wiki you can trust.

**Decay-ranked retrieval.** `ao inject --query "..."` returns a token-budgeted packet weighted by recency, citations, and validation history — different from full-text search.

**Automated capture.** Sessions write run packets, citations, and verdicts without anyone being asked. The discipline lives in the tooling.

**Overnight compounding.** The `ao daemon` runs defrag, evolve, compile, and dream against the corpus while you sleep. A SaaS wiki cannot get smarter on its own.

**Self-writing.** Every consumer of the wiki is also a producer of it. That is the inversion that makes the whole thing tractable.

## How to start

Install for your runtime:

```bash
# Claude Code
claude plugin install agentops@agentops-marketplace

# Codex CLI
curl -fsSL https://raw.githubusercontent.com/boshu2/agentops/main/scripts/install-codex.sh | bash
```

Then, inside any repo:

```bash
ao quick-start          # seed .agents/, GOALS.md, AgentOps instructions
```

That is the entry point. After that, work with your agents normally — `/rpi`, `/council`, `/research`, `/vibe`. `.agents/` accumulates without explicit attention. `ao inject --query "..."` returns a decay-ranked, token-budgeted packet whenever you ask.

## What you own at the end

The wiki is yours. Open source forever. Built so you own the asset, not the tool.

`.agents/` is plain markdown in your repo. If a frontier vendor ships native equivalents next year, the corpus carries forward. If AgentOps changes direction, your corpus is yours. If you outgrow this tool entirely, fork it, customize it, replace it — the corpus is what matters.

For the deeper view of how the same mechanism doubles as a trust factory — identity, reproducibility, evaluation, evidence, recovery — see [trust-factory.md](./trust-factory.md). For the full README, see [the project README on GitHub](https://github.com/boshu2/agentops/blob/main/README.md).
