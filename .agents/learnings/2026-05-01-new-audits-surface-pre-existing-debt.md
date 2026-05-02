---
id: learning-2026-05-01-new-audits-surface-debt
type: learning
date: 2026-05-01
category: process
confidence: high
maturity: provisional
utility: 0.6
source_epic: soc-irg1
---

# Learning: New audit tools surface pre-existing debt; classify before treating as a regression

## What We Learned

I2 shipped `ao skills check` — a new Go-native audit subcommand. On its first run against the repo, it flagged 6 broken references in `discovery/rpi/validation` SKILLs pointing at a non-existent `references/strict-delegation-contract.md`.

These broken refs predate epic soc-irg1. The new audit catches them on first run because no prior audit existed for this surface. The correct classification:
- **NOT a regression** introduced by soc-irg1
- **IS a real bug** worth filing as a separate hygiene issue
- **MUST NOT block** the epic that introduced the audit

Vibe report explicitly classified it as `informational` and recommended a follow-up bd issue rather than failing the vibe gate.

## Why It Matters

Two anti-patterns this prevents:
1. **Audit-introducing-epic gets blocked by debt it catches.** The new tool is the value. Failing to ship it because it surfaces pre-existing problems is self-defeating — the audit can't help future work if it's not in main.
2. **Pre-existing debt gets quietly absorbed into the introducing-epic's scope.** Worker writes a fix for the broken refs while shipping the audit, scope creeps, plan estimate explodes. The right move is: ship the audit, file follow-up, fix in a separate cycle.

Heuristic for classification:
- Does the finding refer to a file/path that this epic touched? → If NO, it's pre-existing.
- Did the finding exist before any commit in this epic? `git blame` or `git log -S` confirms.
- Is the audit tool itself new in this epic? → Then everything it catches on first run is legacy unless proven otherwise.

## Source

Epic soc-irg1, I2 (`ao skills check`). Vibe report `.agents/council/2026-05-01-vibe-soc-irg1.md` Specific Concerns §6. Suggested follow-up command embedded in vibe report.

## Applies When

- Shipping a new linter, audit, validator, or contract checker
- The new tool surfaces issues outside the epic's stated scope
- The issues are real bugs (not false positives from the new tool)

## Counter-applies

- The new tool surfaces issues IN the epic's own diff → those ARE regressions, fix them
- The new tool has known false positives → fix the tool first, then evaluate findings
