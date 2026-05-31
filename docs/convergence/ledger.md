---
title: "AgentOps Convergence Ledger"
description: "Chronological record of external parties — labs, vendors, methodologies — independently arriving at structural conclusions AgentOps already encodes. The receipts of being early."
permalink: /convergence/ledger
last_reviewed: 2026-05-29
---

# Convergence Ledger

> **The receipts.** The dated table behind [The Reading](the-reading.md): each time a major lab, vendor, or methodology independently lands on a structural conclusion already encoded in our doctrine. Distinct from the [Competitive Radar](../comparisons/competitive-radar.md), which maps *competitors in the same lane*. The ledger maps **vindication**: who got here, when, and which of our invariants they reinvented. The argument for *why* this is happening lives in [The Reading](the-reading.md).
>
> **Why keep it.** A thesis you can't date is a thesis you can't defend. When AgentOps says "AI velocity breaks human-paced practice → restructure around bounded autonomous loops," the strongest evidence is that independent teams keep reaching the same place from different starting points. Each entry is a receipt.
>
> **The rule for an entry.** It must be (a) dated, (b) external (not us), (c) a *structural* convergence on a named AgentOps invariant — not merely "they also use agents." Cite the source and the specific invariant. Honesty over hype: where they are *ahead* of us, say so.

## The invariants being converged on

The recurring four (from [`docs/3.0.md`](../3.0.md)):

1. **Velocity breaks human-paced practice** — AI code volume outpaces line-by-line review.
2. **No self-grade** — the agent that does the work never grades it; verification harnesses are independent.
3. **Knowledge becomes a constraint** — learning is durable only once it compiles into a gate/test/rule; deterministic enforcement over trust.
4. **Reasoning core, deterministic boundary** — a probabilistic agent wrapped by a human-governed enforcement layer it cannot bypass.

Plus the supporting shape: spec/BDD-as-plan, eval-data-as-moat, humans up the abstraction ladder, pulled (not ambient) context, immutable provenance.

## Ledger

| Date | Who | What they released / said | Invariant(s) they converged on | Relative to us | Detail |
|---|---|---|---|---|---|
| **2026-05** | **Google SRE** (Papapanagiotou, Malesevic, Heiser, Meshenberg) | Whitepaper: *"AI in SRE: How Google is Engineering the Future of Reliable Operations"* | **All four** + abstraction-ladder, eval-as-moat, spec-as-plan, decoupled reasoning/execution, immutable CoT provenance | Even (8/11) — **ahead on fix-forward** (Intervening-PR Problem) | [Encoding map](google-sre.md) · `ag-4hf7` |
| **2025-10** | **obra/superpowers** (Jesse Vincent) | Claude Code methodology plugin; [launch post](https://blog.fsck.com/2025/10/09/superpowers/), [v4](https://blog.fsck.com/2025/12/18/superpowers-4/) (2025-12) | **No self-grade** + spec/BDD-as-plan + abstraction-ladder | **Even** | Dispatches a *fresh subagent per task* with two-stage review (spec-compliance, then code-quality) — implementer ≠ validator by construction. v4 added a *separate* spec-compliance reviewer, hardening the split. "Design before code, tests before features." |
| **2025-10** | **EveryInc/compound-engineering-plugin** (Every / Kieran Klaassen) | "Compounding engineering" plugin; [repo](https://github.com/EveryInc/compound-engineering-plugin), [concept (2025-06)](https://every.to/source-code/compound-engineering-the-definitive-guide) | Context-as-compounding-artifact + spec-as-plan | **Behind** on knowledge→constraint | Loop "brainstorm → plan → work → review → **compound**"; `/ce-compound` documents learnings so the next agent doesn't re-learn. Real convergence on *context compounding* — but learnings land as **prose notes, not executable gates**. We compile knowledge into a CI gate; they don't. The cleanest place we're ahead. |
| **2025-09 / 10** | **Anthropic** | [Claude Agent SDK guidance](https://claude.com/blog/building-agents-with-the-claude-agent-sdk) (2025-09) + [Agent Skills](https://claude.com/blog/skills) (2025-10) | **Reasoning core + deterministic boundary** (SDK) + context-as-artifact (Skills) | **Even** (narrower) | SDK names the loop "gather context → act → **verify** → repeat" and *ranks* verification: **rules-based feedback best**, LLM-as-judge last & "less robust" — a probabilistic core wrapped by a deterministic check. Skills = portable, composable context folders ("build once, use across Claude apps, Code, API") — the primitive our corpus generalizes. Anthropic ships the primitives; we compose the compounding-corpus + knowledge-as-gate system. |

> **Reading the dates.** The 2025-10 cluster (superpowers, compound-engineering) and Anthropic's 2025-09/10 primitives all *predate* the Google SRE paper (2026-05) but *postdate* the AgentOps CDLC doctrine they converge on. The pattern is independent rediscovery accelerating through late 2025 into 2026 — see [The Reading](the-reading.md) for the mechanism.

## Backfill candidates (verify before promoting)

Tracked-but-not-yet-confirmed convergence moments. Each needs a dated source and a specific invariant before it becomes a ledger row — do not assert without the receipt:

- *(The first wave — superpowers, compound-engineering, Anthropic — is now confirmed and promoted above. Add the next markers here as they surface; promote on a dated, citable receipt.)*

## How to add an entry

1. Confirm it clears the entry rule (dated, external, structural, cited).
2. Add a row to **Ledger** (newest at top). Name the specific invariant(s), not "they use AI."
3. If it surfaces work for us (a gap they expose, a better name to adopt), file a bead and link it.
4. If a detailed mapping is warranted, write a `comparisons/<party>-convergence.md` and link it from the row (as the Google entry does).
5. Update `last_reviewed`.

---

*Companion: [Competitive Radar](../comparisons/competitive-radar.md) (competitors) · [3.0 north star](../3.0.md) (the thesis being converged on).*
