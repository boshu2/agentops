# expert-council

Convene named expert personas to adversarially debate and decide a hard question, then tally a binding score matrix.

## Codex Execution Profile

1. Treat `skills/expert-council/SKILL.md` as the canonical contract.
2. Confirm the 2-4 persona slate with the operator before spawning anything.
3. Run the dueling route in order: independent verdict → 0-1000 cross-scoring → the reveal → score-matrix synthesis.
4. Spawn persona agents via `spawn_agent` (or NTM panes); each persona reads `BRIEFING.md` from disk.

## Guardrails

1. The personas decide; you only count — keep your opinion to a meta-analysis section.
2. Never skip the reveal — it is where the concessions happen.
3. Resolve conflict by the score matrix; keep dissent verbatim, never averaged.
4. Persist all round artifacts to `.agents/council/<topic>-<date>/`.
