---
title: "AgentOps Competitive Radar"
description: "Current market read for AgentOps against the Claude Code skills/plugin ecosystem and adjacent agent-workflow tools."
permalink: /comparisons/competitive-radar
last_reviewed: 2026-05-07
---

# Competitive Radar

AgentOps should not try to be every agent workflow tool at once. The Claude
Code skills/plugin ecosystem has consolidated into five lanes; the strongest
position is narrower and harder to copy than any of them: **a wiki for your
agents, version-controlled in your repo, that compounds across sessions**.
The corpus is the moat. The tool is replaceable.

## Source Set

| Source | Lane | Link |
|--------|------|------|
| AgentOps | Context library / wiki for agents (this doc) | [boshu2/agentops](https://github.com/boshu2/agentops) |
| obra/superpowers | Methodology (TDD discipline, autonomy patterns) | [obra/superpowers](https://github.com/obra/superpowers) |
| EveryInc/compound-engineering-plugin | Methodology (ideate→compound, 7-phase loop) | [EveryInc/compound-engineering-plugin](https://github.com/EveryInc/compound-engineering-plugin) |
| anthropics/claude-plugins-official | Curation (first-party marketplace) | [anthropics/claude-plugins-official](https://github.com/anthropics/claude-plugins-official) |
| jeremylongshore/claude-code-plugins-plus-skills | Volume marketplace (CCPI manager) | [jeremylongshore/claude-code-plugins-plus-skills](https://github.com/jeremylongshore/claude-code-plugins-plus-skills) |
| alirezarezvani/claude-skills | Volume + breadth (skills × platforms) | [alirezarezvani/claude-skills](https://github.com/alirezarezvani/claude-skills) |
| affaan-m/everything-claude-code | Cross-harness (DRY parity across runtimes) | [affaan-m/everything-claude-code](https://github.com/affaan-m/everything-claude-code) |
| trailofbits/skills | Vertical authority (security domain) | [trailofbits/skills](https://github.com/trailofbits/skills) |

## Market Read

The five-lane consolidation (see *Lane segmentation* below) means most
"compete with AgentOps" framings are category errors. A volume marketplace
sells inventory; a methodology plugin sells discipline; a vertical-authority
collection sells domain credibility. None of the seven sells **persistent
context that compounds across sessions on your hardware**. That lane is
empty, and the rest of this radar is about staying in it.

## Seven-competitor lane table

What each competitor wins on (the structural advantage AgentOps cannot beat
them on) and what AgentOps wins on against them. Sourced from the
2026-05-07 council research (`.agents/council/2026-05-07-research-readme-positioning.md` in the repo).

| Competitor | Lane | What they win on | What AgentOps wins on |
|------------|------|------------------|------------------------|
| [obra/superpowers](https://github.com/obra/superpowers) | Methodology | Official Anthropic marketplace placement, ~29K stars, Jesse Vincent brand, TDD red-green-refactor as a sharp methodology hook | Persistent corpus accumulating across sessions; cross-session memory in `.agents/`; multi-model council as a commit gate; off-API daemon on your hardware |
| [EveryInc/compound-engineering-plugin](https://github.com/EveryInc/compound-engineering-plugin) | Methodology | 10-target runtime conversion CLI; configurable per-project reviewer routing; ideation-to-compound surface area; closest philosophical neighbor | Model-independent **per-phase** routing (Claude for discovery, Codex for implementation, fresh Claude for validation, local model for overnight defrag — in one workflow); persistent corpus that lives in your repo, not the tool |
| [anthropics/claude-plugins-official](https://github.com/anthropics/claude-plugins-official) | Curation | First-party authority; default discovery channel; "trust before installing" posture | Operates underneath any harness — Claude Code, Codex, Cursor, OpenCode — turning sessions into a context library you own. AgentOps is not a coding harness; it sits on top of whichever harness you already use |
| [jeremylongshore/claude-code-plugins-plus-skills](https://github.com/jeremylongshore/claude-code-plugins-plus-skills) | Volume marketplace | 425 plugins / 2,810 skills inventory; CCPI package manager; sponsored placement; daily download metrics | Compounding context vs. static inventory: skills don't accumulate, a wiki does. AgentOps ships a bookkeeping schema that grows; volume marketplaces ship an inventory that doesn't |
| [alirezarezvani/claude-skills](https://github.com/alirezarezvani/claude-skills) | Volume + breadth | ~5,200+ stars; 235 skills × 12 platforms; 305 stdlib Python tools; multi-domain coverage (engineering + marketing + compliance) | Same persistent-corpus argument plus model-independent phase routing inside one session — breadth of skills doesn't replace cross-session memory |
| [affaan-m/everything-claude-code](https://github.com/affaan-m/everything-claude-code) | Cross-harness | DRY parity across 5 runtimes; 48 subagents; "Anthropic Hackathon Winner" credential. (README also claims 140K+ stars / 21K forks; the entire Claude Code ecosystem is below those numbers — flagged as not validated.) | Cross-runtime *distribution* is not cross-model *per-phase routing*. AgentOps mixes models per phase within one RPI loop, with state preserved across boundaries — a different optimization |
| [trailofbits/skills](https://github.com/trailofbits/skills) | Vertical authority (security) | Trail of Bits brand; security skills + a "Trophy Case" of CVE-shaped findings; domain credentialing no general-purpose tool can match | Different category — AgentOps is the substrate a vertical-authority collection runs on. Their skills can live inside an AgentOps corpus; the inverse is not true |

**What this table reveals:** none of the seven are selling persistence,
sovereignty, off-API operation, multi-model per-phase routing, or
context-as-a-discipline. The substrate / wiki-for-agents lane is empty if
AgentOps claims it.

## Lane segmentation

The Claude Code skills/plugin ecosystem has consolidated into five lanes.
Each row names the lane, the canonical examples, the buyer signal, and
AgentOps's posture toward it (compete, complement, or ignore).

### 1. Volume (marketplace / many skills)

**Examples:** [jeremylongshore/claude-code-plugins-plus-skills](https://github.com/jeremylongshore/claude-code-plugins-plus-skills) (425 plugins, 2,810 skills, CCPI manager, 100-point grading); [alirezarezvani/claude-skills](https://github.com/alirezarezvani/claude-skills) (235 skills × 12 platforms); [affaan-m/everything-claude-code](https://github.com/affaan-m/everything-claude-code) (182 skills, 48 subagents) overlaps here too.

**Buyer signal:** "We have the most stuff." Inventory bulk as the value prop. Costco bulk-buy of pre-built skills.

**AgentOps's posture:** **Complement, do not compete.** AgentOps is a different category — a context library / wiki, not a skills inventory. The two combine: install the volume marketplace for breadth; run AgentOps for the corpus discipline that turns those skills' outputs into persistent context.

### 2. Methodology (workflow / discipline)

**Examples:** [obra/superpowers](https://github.com/obra/superpowers) (TDD red-green-refactor); [EveryInc/compound-engineering-plugin](https://github.com/EveryInc/compound-engineering-plugin) (ideate→compound, 7-phase loop, 10-target conversion).

**Buyer signal:** "Use our workflow and your agents will produce better code." A sharp opinion about *how* to drive agents.

**AgentOps's posture:** **Compete obliquely, do not contest head-on.** This lane is structurally hard to win — Superpowers has 29K stars and official-marketplace placement; Compound Engineer has the closest philosophical positioning. Winning the methodology lane is *not* AgentOps's bet. AgentOps offers methodology surfaces (`/rpi`, `/council`, `/pre-mortem`, `/vibe`) but its claim is one level deeper: methodology sits on top of context, and the context is what compounds. Anthropic's Managed Agents (May 2026) is also moving into this lane natively.

### 3. Vertical (specific domain)

**Examples:** [trailofbits/skills](https://github.com/trailofbits/skills) (security skills + Trophy Case of findings).

**Buyer signal:** "We are the authoritative source for skills in domain X." Domain credibility no general-purpose tool can replicate.

**AgentOps's posture:** **Complement.** Vertical collections produce skills; AgentOps produces the context library those skills run inside. A security team can install Trail of Bits skills on top of an AgentOps corpus; the security findings then flow into `.agents/` as persistent learnings. AgentOps does not chase vertical authority — that's a brand investment, not a tool feature.

### 4. Curation (small high-quality set)

**Examples:** [anthropics/claude-plugins-official](https://github.com/anthropics/claude-plugins-official) (first-party marketplace, "trust before installing"); selective collections that prioritize quality over volume.

**Buyer signal:** "We curate so you don't have to." Trust + discovery channel as the value prop.

**AgentOps's posture:** **Ignore as a competitor; respect as distribution.** First-party curation is unbeatable for trust and discovery, and the right move is to be installable through it rather than to compete with it. AgentOps's claim is orthogonal: it's not "trust this skill" but "build a corpus that survives whichever skill you used last week."

### 5. Cross-harness (multi-runtime parity)

**Examples:** [affaan-m/everything-claude-code](https://github.com/affaan-m/everything-claude-code) (DRY parity across Claude Code, Cursor, Codex, OpenCode, Gemini); [EveryInc/compound-engineering-plugin](https://github.com/EveryInc/compound-engineering-plugin) (10-target conversion CLI) overlaps here.

**Buyer signal:** "Our skills run everywhere." Multi-runtime distribution as the value prop.

**AgentOps's posture:** **Compete on a sharper claim.** Cross-harness *distribution* is multi-runtime spread of one workflow shape. AgentOps does cross-harness *and* model-independent **per-phase routing** — Claude does discovery, Codex implements, fresh Claude validates, an open-weights local model handles overnight defrag, all in one RPI loop with state preserved across boundaries. Nobody else in the seven-competitor set does this. The pitch is "mix and match models per phase," not "the same skill on five runtimes."

## Where AgentOps Wins

Six differentiators no competitor in the seven-competitor set has. Sourced from
the 2026-05-07 council research (`.agents/council/2026-05-07-research-readme-positioning.md` in the repo).

### 1. Persistent corpus

A bookkeeping schema that *grows*: learnings, patterns, planning rules, and
cited decisions accumulate in `.agents/` as plain markdown, version-controlled
with the code. Competitors ship skills (static inventory); AgentOps ships the
discipline that turns sessions into a wiki. Receipts: as of 2026-05-04, this
repo's `.agents/` contained ~1,842 learnings, ~186 patterns, ~80 planning
rules, and ~3,867 cited decisions captured by the system on itself.

### 2. Off-API daemon

`ao schedule` + `ao daemon` runs dream / evolve / compile / defrag / forge
overnight, off-vendor, on your hardware, against your subscription. All seven
competitors are in-session plugins. The daemon is the structural answer to
"what if a frontier vendor ships native equivalents in 12 months" — your
corpus and your scheduler keep running regardless.

### 3. Multi-model council

`/council --mixed` runs Claude + Codex (and other) judges in parallel against
one evidence packet, producing a verdict before commit. Compound Engineer has
reviewer agents but they are single-model and post-implementation. A
multi-model commit gate is the strongest validation primitive in the set.

### 4. Model-independent phase routing

Pick Claude for ideation, Codex for validation, an open-weights local model
for overnight defrag — *per phase, in one workflow,* with state preserved
across the boundaries. Cross-harness *distribution* (everything-claude-code,
Compound Engineer's 10-target conversion) is multi-runtime spread of one
workflow shape; per-phase routing is mixing models inside one workflow.
Different optimization. Nobody in the seven does it.

### 5. Context-engineering vocabulary

AgentOps owns "wiki for agents" + "context library" + "context compiler"
+ "CDLC" as a coherent vocabulary, anchored by the SE → context translation
table (source code → context, SDLC → CDLC, libraries → context libraries,
compilers → context compilers, code review → multi-model councils, CI/CD →
validation gates, postmortems → automated postmortems, runbooks → skills +
planning rules, software factories → software factory daemon, Markdown/Git
/Linux → LLM Wiki of Markdown, open-source corpus → your private corpus).
Vocabulary ownership is durable; Superpowers owns "TDD," AgentOps can own
"context engineering."

### 6. Honest empirical disclosure

Δ=+0.0000 at workbench v1 difficulty, published in-repo. Independent 3-judge
audit (2026-05-06) confirmed parity with Anthropic Managed Agents on rubric
authoring, separate-context grading, and iterate-until-pass. In a market of
inflated star counts and ungraded "100-point validations," a published null
result and a transparent audit are credibility assets.

## Current Vulnerabilities

| Vulnerability | Impact | Best next move |
|---------------|--------|----------------|
| Corpus durability under routine cleanup | The receipts claim ("1,842 learnings") becomes fragile if maintenance can wipe `.agents/` subdirs (observed 2026-05-07). | Snapshot/restore mechanism + tracked durability fix; in the meantime, receipts cite the 2026-05-04 stable snapshot with timestamp. |
| Methodology lane is being eaten | Anthropic's Managed Agents (May 2026) and Superpowers' marketplace placement compress the methodology buyer's choice set. | Stay out of the methodology lane head-on; lead with the wiki framing and lane-segmentation argument. |
| Compounding proof is still too implicit | Users have to trust the flywheel story before they feel it. | Put Dream reports, `ao demo`, and corpus-stats in the first-run path. |
| Reviewer routing is less configurable than Compound Engineer | CE can feel more tailored to a stack. | Document per-project validation profile selection; expose council config more visibly. |
| Volume-marketplace shoppers may bounce off "73 skills" framing | Inventory-comparison buyers will not see the corpus advantage. | Keep the volume comparison explicit (vs-tons-of-skills doc); reframe the question from "how many skills" to "what does the asset look like in 6 months". |

## Execution Bias

Do not respond to every competitor feature by adding another command. Favor
moves that make the wiki visible, automatic, and verifiable:

1. Make the corpus inspectable on day one (`scripts/corpus-stats.sh`, `ao inject` traces).
2. Make first value obvious in under five minutes (skills install + first session writes to `.agents/`).
3. Keep the per-phase model-routing demo as the anchor (it's the killer feature buried in the founder pitch).
4. Defend the wiki/context-engineering vocabulary in every doc; do not drift to "skills repo" or "methodology" framings.
5. Keep comparison docs tied to current official sources; re-run `scripts/check-competitive-freshness.sh` before each release.
