<!--
Frozen at Wave 0 closure of the Reconciliation Engine epic (soc-9xn0).
Closure SHA:  ab479e26 (docs(positioning): wiki-framing sweep across all surface docs)
Closure date: 2026-05-07T20:31:47-04:00
Captured by:  PR-E (soc-mt50) on 2026-05-08, epic soc-xlw8.

This file freezes the README/PRODUCT/GOALS hero sections that defined the
positioning thesis at Wave 0 close. The Thesis-Stability Gate (soc-r3y8b)
between Wave 1 and Wave 2 of the Reconciliation Engine arc diffs the
*current* hero sections against this snapshot. Drift = thesis shift =
operator must explicitly accept (and re-validate Waves 2-4) or re-brainstorm.

Extraction contract — must match scripts/check-thesis-stability.sh exactly:
  awk 'NR==1, /^## / {if (!/^## /) print}' <file>

Each section below is the verbatim awk output for that file at the closure SHA.
DO NOT hand-edit these blocks. To regenerate, run:
  scripts/check-thesis-stability.sh --rebuild-snapshot <SHA>
(and revert in git if the rebuild reveals an unintended thesis change).

DO NOT add a `<!-- WAVE_0_TODO -->` marker to this file unless the snapshot
is genuinely incomplete — the script exits 2 (precondition error) when it
finds that marker. Per pre-mortem L6.
-->

# Wave 0 Thesis Snapshot

Frozen at SHA `ab479e26` on 2026-05-07T20:31:47-04:00 (soc-9xn0 closure).

## README.md hero

```
<div align="center">

# AgentOps

[![GitHub stars](https://img.shields.io/github/stars/boshu2/agentops?style=social)](https://github.com/boshu2/agentops/stargazers)

### A wiki for your agents. Built so you own the moat.

`.agents/` is just a wiki — markdown files in your repo, version-controlled with your code, that agents read, traverse, and contribute to. The kind of wiki your team should already have. AgentOps automates the discipline of building one.

*The only verifiable moat in this uncertain time is context. Models will get smarter, harnesses will commoditize, agents will get cheaper. Your accumulated context — the lessons learned about your individual problems, the patterns that worked, the decisions that survived review — is the one asset that compounds and doesn't get eaten by the next vendor release. That's what your company actually is.*

*AgentOps is the shovel. Start digging.*

> AgentOps is not a coding harness. The labs are building those, and they will keep getting better. AgentOps sits on top of whichever harness you already use — Claude Code, Codex, Cursor, OpenCode — and turns your business, codebase, and team practices into a context library those agents mix and match from. Mix and match Claude, Codex, or any model at every phase. Lives in `.agents/` in your repo. Runs on your hardware. Evolves with the models.

*AgentOps was used to develop AgentOps. As of 2026-05-04, this repo's `.agents/` directory contained ~1,842 learnings, ~186 patterns, ~80 planning rules, and ~3,867 cited decisions captured by the system on itself across thousands of phase transitions. Re-run anytime: `bash scripts/corpus-stats.sh`. Independent 3-judge audit (2026-05-06) confirmed parity with Anthropic Managed Agents on rubric authoring, separate-context grading, and iterate-until-pass.*

</div>

---

```

## PRODUCT.md hero

```
---
last_reviewed: 2026-05-07
---

# PRODUCT.md

```

## GOALS.md hero

```
# Goals

A wiki for your agents — repo-native, version-controlled, mechanically maintained — that turns your context into the durable moat under any model or harness.

```
