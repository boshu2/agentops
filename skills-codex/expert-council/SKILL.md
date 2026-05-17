---
name: expert-council
description: 'Convene named expert personas to adversarially debate and decide a hard question.'
---
# $expert-council — Adversarial Expert Council (Codex Native)

Convene 2-4 named domain-expert personas as parallel agents. Each renders an independent verdict on one hard decision; they then adversarially cross-score and rebut each other 0-1000; you tally a binding score matrix. Disagreement is signal — where personas agree the call is sound, where they trash each other the gap is information.

Canonical specification: `skills/expert-council/SKILL.md` in the host repo. This file is the Codex-first execution contract.

## When to use

A high-stakes, multi-lens decision (strategy, positioning, architecture, career) with no single right answer, where the operator can name 2-4 experts they trust. Not for PASS/WARN/FAIL validation of a concrete artifact — use `$council` for that.

## Critical constraints

1. **Personas decide; you only count.** Tally the cross-scores and report them faithfully — your opinion belongs only in a meta-analysis section.
2. **The briefing goes on disk.** Write one `BRIEFING.md`; persona agents read it from the filesystem. Never relay a large briefing inline.
3. **The reveal is mandatory.** Never stop after cross-scoring — the reveal is where personas confront the score gaps and concede.
4. **Spawn in a clean workspace**, not the host repo, so host instructions do not derail the persona agents.

## Workflow (the dueling route)

| Phase | Action |
|-------|--------|
| 1 | Pick the persona slate (2-4 named experts); confirm with the operator. |
| 2 | Spawn one agent per persona (`spawn_agent`, or NTM panes). |
| 3 | Write `BRIEFING.md` — subject, options, honest read, and a mandatory "tension you must confront". |
| 4 | Each persona writes an independent verdict in character — ranked recommendations, kill list, positioning, #1 risk. |
| 5 | The Duel — each persona scores every other verdict 0-1000, per recommendation, candidly. |
| 6 | The Reveal — show each persona how the others scored it; collect concessions; run the blind-spot probe. |
| 7 | Synthesis — build the score matrix; rank the binding decision by cross-score; record contested calls and mutual kills. |

Checkpoints: confirm the slate before spawning; verify every persona file exists before advancing a phase; if cross-scores all cluster above 800, nudge for candor and re-score.

## Output

Markdown. `DUEL.md` (the score-matrix report) plus `BRIEFING.md` and per-round persona files, persisted to `.agents/council/<topic>-<date>/`. Structure: executive summary, whole-ballot score matrix, consensus winners (the binding decision), contested calls, mutual kills, blind spots, meta-analysis.

## Guardrails

1. Keep persona verdicts structured: ranked recommendations, kill list, one-sentence positioning, #1 risk.
2. Resolve conflict by the score matrix, not by averaging — keep dissent verbatim.
3. Each persona argues in character with its real frameworks; no flattery.

See `prompt.md` in this directory for the condensed execution profile.
