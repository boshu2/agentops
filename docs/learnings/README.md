# Learnings Index

Durable observations and rules-derived-from-mistakes extracted from
live sessions. Each file is a single learning; the date prefix is the
day it was captured.

For canonical contracts (what the system SHOULD do), see
`../contracts/`. For research deep-dives, see `../brainstorms/` or
`../research/`. Learnings are the "what we learned by running"
layer in between.

## Catalog (chronological, newest first)

| Date | File | One-line takeaway |
|---|---|---|
| 2026-05-13 | [cli-wiring-cycle-shape.md](2026-05-13-cli-wiring-cycle-shape.md) | CLI-wiring is a repeatable ~10-min cycle-shape: parent noun + verb subcommand + injectable func; 3 production adapters exposed in 3 cycles. |
| 2026-05-13 | [loop-context-drift-87-cycle-observation.md](2026-05-13-loop-context-drift-87-cycle-observation.md) | `/loop` context accumulates but disk-state ledger protects correctness — soc-wx55q.1 P1 may be P3. |
| 2026-05-13 | [substring-sed-rename-overreach.md](2026-05-13-substring-sed-rename-overreach.md) | Before any bulk `sed` rename, enumerate ALL substring-containing identifiers and classify by concept. |
| 2026-05-13 | [bc-ports-wire-up-arc.md](2026-05-13-bc-ports-wire-up-arc.md) | 12 production adapters in 11 cycles via repeatable file-triplet pattern; 8 distinct adapter shapes proven. |
| 2026-05-12 | [parity-surface-inventory-grew-from-4-to-7-across-cycles-64-70.md](2026-05-12-parity-surface-inventory-grew-from-4-to-7-across-cycles-64-70.md) | The parity surface count between Claude and Codex grew incrementally; tracking the count caught drift. |
| 2026-05-11 | [evolve-skill-friction-from-13-cycle-session.md](2026-05-11-evolve-skill-friction-from-13-cycle-session.md) | 6 concrete `skills/evolve/SKILL.md` patches identified from direct cycle-by-cycle observation. |
| — | [orchestrator-compression-anti-pattern.md](orchestrator-compression-anti-pattern.md) | Don't collapse `/research → /plan → /pre-mortem` into a single inlined call; strict delegation matters. |

## When to add a learning here

Add a file when one of these is true:

1. **A long arc landed and the lessons are non-obvious.** Example:
   the cycle-122 BC-ports wire-up arc (12 cycles, 8 adapter shapes
   proven) — future operators reading just the commit messages
   would miss the meta-pattern. The arc summary captures it.
2. **A mistake produced a rule.** Example: the cycle-127 substring-
   sed overreach learning. The mistake was caught, the corrective
   commit landed, and the rule that prevents repeating it is now
   durable.
3. **Empirical observation refines a prior theoretical concern.**
   Example: cycle-132 documented 87 fires of `/loop` showing the
   feared context-drop didn't materialize because disk state
   protects correctness — refining soc-wx55q.1.

Do NOT add a file for:
- One-off bug fixes (the commit message is enough)
- Already-canonical-contract material (put it in `../contracts/`)
- Status updates on in-progress work (use `bd comment` or
  `.agents/evolve/cycle-history.jsonl`)

## File shape

```markdown
---
title: <single-sentence statement of the learning>
date: YYYY-MM-DD
tags: [topic1, topic2, ...]
source: <where the observation came from — cycle range, incident, PR>
companion: <optional sibling file>
---

# <repeat title>

<short paragraph: what happened>

## What went wrong / What we learned

<concrete details>

## The rule

<numbered or bulleted, actionable for future cycles>

## See also

<other learnings, contracts, beads>
```

Frontmatter is required so `ao lookup` and future indexers can rank.
The body shape is a guideline, not a contract — match what the
observation demands.
