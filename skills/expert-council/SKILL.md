---
name: expert-council
description: |
  Convene named domain-expert personas to adversarially debate and decide a hard question via an NTM swarm.

  **Use when:**
  - You face a high-stakes, multi-lens decision (strategy, positioning, architecture, career) with no single right answer
  - You can name 2-4 experts whose perspectives you trust and want them to duel, not merely agree
  - You want disagreement surfaced as signal — a binding ranked verdict with recorded dissent, not a polite consensus

  **Triggers:** "expert council", "convene a council", "dueling council", "have <experts> decide", "council of <names>"

  **Not ideal for:**
  - PASS/WARN/FAIL validation of a concrete artifact — use /council (evidence-packet judges) instead
  - Single-answer factual questions, or decisions with an obvious conventional default
skill_api_version: 1
user-invocable: true
context:
  window: inherit
  intent:
    mode: task
  sections:
    exclude: [HISTORY]
  intel_scope: topic
metadata:
  tier: judgment
  dependencies: []
  stability: experimental
output_contract: ".agents/council/<topic>-<date>/DUEL.md — score-matrix report; rounds/ per-persona files"
---

# /expert-council — Adversarial Expert Council

Convene 2-4 named domain-expert personas in an NTM swarm. They independently render verdicts on one hard decision, then **adversarially cross-score and rebut each other 0-1000**; the orchestrator tallies a binding score matrix. Where personas strongly agree, the call is sound; where they trash each other, the gap is pure information.

## Overview

This skill runs the `dueling-idea-wizards` route on a *decision* rather than a feature backlog. The operator picks experts they trust — living or historical (Kent Beck, Kelsey Hightower, a composite VC, …). Each persona argues in character with its real frameworks. The **reveal** — showing each persona how the others scored *their* verdict — forces honest concessions; the **blind-spot probe** surfaces what no single persona saw. The output is a ranked decision with the dissents kept, not averaged away.

Distinct from two cousins:
- **vs `/council`** — that runs fresh-context judges over an *evidence packet* for a PASS/WARN/FAIL gate. This runs *persona advocates who duel and score* to produce a ranked strategic verdict.
- **vs `dueling-idea-wizards`** — same cross-scoring engine, but the duelists are named domain experts and the scored unit is a strategic recommendation, not a feature idea.

## ⚠️ Critical Constraints

- **Personas decide; the orchestrator only counts.** Synthesis tallies the cross-scores and reports them faithfully — operator opinion belongs only in a meta-analysis section. **Why:** an editorializing orchestrator silently overrides the council the operator convened.
- **The briefing goes on disk, never through argv.** Write one `BRIEFING.md`; persona agents read it from the filesystem. **Why:** a >~128 KB argv string triggers `E2BIG`; relaying a large briefing inline fails silently mid-swarm.
- **The reveal is mandatory.** Never stop after cross-scoring. **Why:** the reveal is where personas confront the score gaps and concede — skipping it discards the highest-signal output of the run.
- **Codex on a ChatGPT-billed account cannot join an NTM swarm.** NTM rejects `gpt-*-codex` model IDs; OpenAI rejects non-codex IDs. **Why:** the pane spawns, looks alive, and silently rejects the first prompt. Run all-Claude personas, or give Codex a real API key — persona diversity matters more than model diversity.
- **Spawn in a clean workspace under `projects_base`, not the host repo.** **Why:** host-repo `CLAUDE.md` and SessionStart hooks derail persona agents into unrelated repo work.

## Workflow

Detailed per-phase prompt templates and NTM commands: [references/dueling-route.md](references/dueling-route.md).

### Phase 1: Pick the persona slate
Confirm 2-4 named experts with the operator (the slate shapes the verdict — never assume it). Map each persona to its real frameworks.
**Checkpoint:** operator confirms the slate before any spawn.

### Phase 2: Spawn the swarm
Resolve `ntm config get projects_base`, create a clean council workspace, `ntm spawn <name>-council` — one pane per persona.
**Checkpoint:** every pane is alive and idle; tail each for `status:400` (Codex model rejection).

### Phase 3: Write the briefing
Author one `BRIEFING.md`: the subject, the options, an honest asset read, and a mandatory **"tension you must confront"** section that forces real conflict.
**Checkpoint:** briefing names a concrete, contestable tension.

### Phase 4: Independent verdict
Each persona reads the briefing and writes `round1-<persona>.md` in character — ranked recommendations, kill list, one-sentence positioning, #1 risk. No cross-talk.
**Checkpoint:** all persona verdict files exist and are non-trivial.

### Phase 5: The Duel — adversarial cross-scoring
Each persona scores every other persona's verdict 0-1000, per recommendation, candidly. Where a judge is wrong, dock hard and say why.
**Checkpoint:** if all scores cluster above 800, nudge for candor and re-score.

### Phase 6: The Reveal + blind-spot probe
Show each persona how the others scored *their* verdict; collect honest concessions and rebuttals. In the same pass, ask each: "what did none of us see?"
**Checkpoint:** every reveal file records at least one concession or a defended disagreement.

### Phase 7: Synthesis — the score matrix
Build the matrix: every recommendation × every persona's score. Rank by cross-score, weighting consensus over unilateral enthusiasm. Identify consensus winners, contested calls (operator tiebreak), and mutual kills. Write the report; persist all artifacts into the host repo.

## Output Specification

**Format:** Markdown.
**Filename:** `DUEL.md` (the score-matrix report) plus `BRIEFING.md` and `rounds/round{1}-*.md`, `rounds/scores-*.md`, `rounds/reveal-*.md`.
**Location:** persisted to `.agents/council/<topic>-<date>/` in the host repo.
**Structure:** executive summary · methodology · whole-ballot score matrix · consensus winners (the binding decision, ranked by cross-score) · contested calls · mutual kills · blind spots · meta-analysis of each persona's scoring bias.

## Quality Rubric

- [ ] 2-4 named personas, each argued in character with its real frameworks
- [ ] Briefing on disk with an explicit, contestable "tension to confront"
- [ ] Cross-scores are candid (not all clustered high) and per-recommendation
- [ ] The reveal ran — every persona saw its scores and conceded or defended
- [ ] Score matrix ranks the binding decision by cross-score, not by vote count
- [ ] Dissents and contested calls are recorded verbatim, not averaged away
- [ ] All round artifacts persisted under `.agents/council/<topic>-<date>/`

## Examples

```bash
/expert-council decide my 90-day founder strategy — slate: Kent Beck, Kelsey Hightower, a composite VC
/expert-council should we adopt event sourcing? — convene Fowler, a staff SRE, and a YAGNI hardliner
```

## Troubleshooting

| Problem | Cause | Solution |
|---------|-------|----------|
| Codex pane rejects the first prompt | ChatGPT-billed Codex; NTM blocks `gpt-*-codex`, OpenAI blocks `gpt-5` | Run all-Claude personas, or give Codex a real API key |
| `ntm spawn` fails silently | Wrong `projects_base` | `ntm config get projects_base`; create the workspace under it |
| `ntm send` hits the operator's shell | `--all` includes the user pane | Target panes with `--pane N`; use `--no-cass-check` for round broadcasts |
| Scores all cluster 800+ | Love-fest — personas avoiding conflict | Nudge: "be more critical; some of these must have weaknesses"; re-score |
| Personas converge with no dissent | Briefing lacks a real tension | Strengthen the "tension you must confront" section; re-run from Phase 4 |

## See Also

- [council](../council/SKILL.md) — evidence-packet judges for a PASS/WARN/FAIL gate (different job)
- [swarm](../swarm/SKILL.md) — parallel agent dispatch without the scoring duel
- [skill-auditor](../skill-auditor/SKILL.md) — audit this skill before declaring it stable
- [references/dueling-route.md](references/dueling-route.md) — per-phase prompt templates and NTM command reference
