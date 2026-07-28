---
id: plan-2026-07-28-skill-overhaul-reboot
type: plan
date: 2026-07-28
status: in_progress
goal: Enhance every canonical skill per the 2026-07-24 overhaul findings, in lean waves, without the proof-epoch machinery
supersedes: plan-2026-07-24-skill-system-overhaul
inputs:
  - docs/plans/2026-07-24-skill-system-overhaul.md
  - docs/audits/2026-07-25-skill-overhaul-progress-review.md
  - docs/audits/2026-07-28-skill-overhaul-reboot/
---

# Skill overhaul reboot

## Why a reboot

The 2026-07-24 plan diagnosed real problems and produced real evidence: a
sealed design duel, four deep per-skill audits covering all 49 canonical
skills, and a Go CLI audit. Its execution model then consumed three days in
proof-epoch bookkeeping — frozen intents, packet digests, transition records,
five-repair-round reviews — while landing almost nothing. The 2026-07-25
progress review measured the result: two of nine tranches proven, zero
branches pushed, ADR-0016 violated at scale on the tranche fleet.

Since then the highest-value fixes landed on `main` independently:

- `16d764b5a` (#988) — `craft-goal` skill added; RPI reports humanized.
- `aceeb6f10` (#995) — the RPI/Validate digest disagreement fixed; ADR-0016
  shipped-Python gate enforced.
- `f66ab953a` (#996) — every blocking gate must prove it can fail.
- `regen-check` is live and green.

What never landed is the actual point of the program: the per-skill
improvements themselves. This plan delivers those, and only those.

## What is abandoned, deliberately

From the 07-24 plan, the following machinery is **not** part of this program.
It was process scaffolding whose cost exceeded its evidence value:

- proof epochs, proof-contract transitions, qualification corpora;
- frozen intent packets, effect receipts, per-tranche durable verdicts;
- the v3 skill compiler shadow rail and one-shot catalog cutover;
- the 30-scenario frozen routing corpus and live-model A/B sample;
- the tranche fleet (fifty-odd worktrees). Branches remain on disk and on
  `origin/codex/skill-overhaul-seed` for salvage reference; nothing new
  builds on them.

Validation for this program is the repository's existing deterministic gates
plus one fresh cross-family review per wave. One round, findings triaged and
fixed, done. A wave that draws two same-class refutes stops and re-scopes
instead of grinding repair rounds.

## What each skill enhancement is

For each skill, using its row in the 07-24 target matrix plus its distilled
audit checklist (`docs/audits/2026-07-28-skill-overhaul-reboot/checklists/`):

1. **Verify first.** Checklist items are hypotheses from a seed-era tree;
   confirm each against the live file before editing.
2. **Fix defects.** Broken script/path references, stale commands, claims
   that contradict live CLI behavior, dead links.
3. **Tighten the contract in place.** One distinct intent; honest placement
   in the Goal-to-RPI loop; explicit authority and effects; positive and
   negative triggers; failure semantics (unavailable, timeout, partial
   evidence); a bounded procedure. All within the existing v2 frontmatter
   schema — no new schema, no new taxonomy axes.
4. **Regenerate projections** through their owners (`regen-all`, codex
   twins, catalog, router) — never hand-edit generated files.

## Waves

Skill groups follow the 07-24 matrix tranches; each wave is one bead, one
branch, one PR. Waves are independent except W8.

| Wave | Skills |
|---|---|
| W1 kernel | rpi, plan, implement, validate |
| W2 product/campaign | product, goals, craft-goal, automation-shape-routing |
| W3 evidence/judgment | cass, codebase-recon, council, domain, idea-genie, postmortem, premortem, reality-check, research, reverse-engineer, scope, security, standards |
| W4 specialists | converter, doc, refactor, scaffold, skill-builder, test, workflow-builder |
| W5 evolution | learn, operationalize, pattern-mining, toil-mining |
| W6 runtime | agent-mail, agent-native, agy-native, codex-exec, ntm, rch, swarm, using-gc |
| W7 support | account-rotation, bootstrap, cc-hooks, dcg, handoff, ms, sbh, shared, status |
| W8 coherence | full regen, SKILL-ROUTER/catalog/tiers coherence, description budgets, closing review |

Structural changes from the 07-24 plan that exceed text enhancement are
separate beads, not wave work:

- `goals` → `fitness` rename with compatibility alias (07-24 D9);
- `shared` semantic retirement (07-24 D9);
- remaining Go CLI G0–G2 items not already landed.

## Wave acceptance

A wave is done when:

- every checklist `[defect]` for its skills is fixed or explicitly rejected
  with a one-line reason in the wave PR;
- every `[enhance]` item is applied, deferred with reason, or rejected with
  reason — no silent drops;
- `make regen-check` and the standard push gates are green;
- one fresh cross-family review of the wave diff ran; its findings are fixed
  and the fix delta gets exactly one re-review. Two rounds is the ceiling —
  residual non-defect nits are recorded in the wave PR, not ground out;
- the wave PR is merged to `main`.

## Program completion

- All eight waves merged.
- The structural beads dispositioned (done or explicitly deferred by Bo).
- `docs/plans/2026-07-24-skill-system-overhaul.md` and this plan both marked
  superseded/complete, with the skill sources themselves as the only
  authority — no plan document owns any live placement fact.
