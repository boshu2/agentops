---
id: research-2026-04-13-option-d-conditional-turn-writer-normalization
type: research
date: 2026-04-13
bead: na-0vyc
source_queue: .agents/rpi/next-work.jsonl
source_epic: option-d-rpi-closeout
finding: CONDITIONAL turn-level content writer + XML strip filter + subagent join
verdict: evidence-only-close
---

# Option D Conditional Turn Writer Normalization

## Question

Should the queued Option D conditional item be built now?

Queue item:

> CONDITIONAL: turn-level content writer + XML strip filter + subagent join

Trigger condition:

> ONLY build if post-ship eval harness shows existing pipeline is inadequate for retrieval.

## Evidence

- `.agents/council/2026-04-11-phase2-decision-inline.md` selected D-MIN and explicitly said not to start W1 code work unless post-ship measurement identifies a specific gap.
- `.agents/rpi/baseline-precision.json` says the strict `0.0` baseline was a measurement artifact caused by narrow labels and frontmatter-only stubs, not proof that retrieval is broken.
- `env -u AGENTOPS_RPI_RUNTIME bash scripts/check-retrieval-quality-ratchet.sh` passed on 2026-04-13 with `any_relevant_at_k=0.65`, threshold `0.60`, and `hits=13/20`.
- `.agents/overnight/latest/retrieval-bench.json` reports live-local coverage `1.0`: 10 of 10 queries returned hits at `k=3` over 231 learnings.
- GitHub Actions Validate run `24354916014` passed `retrieval-quality` on commit `e680f596475d3b188a4d0f99446187d3f2cb1a4e`.
- `skills/sessions/references/tool-noise-filter.md` is not present on current `main`; `docs/preserved-refs.tsv` shows the tool-noise drafts are preserved WIP refs, not accepted mainline specs.

## Pre-Mortem

Risk: closing this item could hide a real retrieval failure.

Counter-evidence: the repo now has the softer retrieval ratchet and CI retrieval-quality job the Option D memo asked for. Both are green. The live-local overnight bench also shows full query coverage. The conditional build should wait for a failing or inadequate measurement, not run on the original speculative plan.

Risk: the preserved tool-noise spec branches could contain useful work.

Counter-evidence: the preserved refs have explicit retirement rules. Their existence is not enough to trigger the conditional writer build. They should be reconciled only if the retrieval signal later fails or a separate bead chooses to retire those preserved refs.

Risk: not building turn-level session files leaves future retrieval blind spots.

Counter-evidence: `.agents/rpi/baseline-precision.json` identifies frontmatter-only pattern stubs and overly strict ground truth as the observed bottlenecks. Those upstream issues were already split into separate consumed work. Building a turn writer now would widen scope without a current failing gate.

## Decision

Do not build the turn-level content writer now. Consume the queued conditional item as evidence-only against `na-0vyc` because the condition is unmet: current retrieval gates pass and the D-MIN memo says to defer W1 code unless measurement proves inadequacy.

Durable proof packet: `.agents/releases/evidence-only-closures/na-0vyc.json`.
