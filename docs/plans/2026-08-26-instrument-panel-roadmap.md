# Instrument-panel roadmap: make skill measurement real, deterministically

> **Status:** EXECUTED 2026-08-28 — Train 1 merged as `8cdcb5a90` (#1087), Train 2 as `e69144d6d` (#1088).
> **Closeout:** every lane landed. One acceptance clause honestly MISSED: L2's
> "≥ 2 skills move off UNMEASURED" finished at zero — the one filed row was
> withdrawn when its fresh validator proved transcript-level control-arm
> contamination, and the harness now degrades any rep whose transcript reads a
> SKILL.md. Named successors: filesystem-sealed dispatch, then re-run the
> premortem/council waves (council needs a second effort level); the
> membrane-receipts dead-generator claim; extend the tightening-ratchet to the
> probe harness. · **Intent source:** this document.
> **Provenance:** 2026-08-23 estate audit → 2026-08-26 four-lane investigation
> (compound-engineering/Every, Matt Pocock skills, gbrain+gstack local, eidetic-engine).
> Verdict being executed: *ao is the deterministic instrument panel; value grows by
> adding gates under `ao gate check`, never new root commands.* Recovered base:
> branch `recover/gstack-clean-room` (9872483bd).
> **Retirement:** superseded when every lane below is merged or explicitly dropped;
> the consumer is the orchestration run that executes it and the fresh validators
> that judge each lane against the acceptance written here.

## Caller-visible outcome (the one behavior)

Today `evals/skill-probes/LEDGER.md` honestly reports **0/12** product/judgment
skills measured, because frontier models saturate both arms of whole-skill A/Bs.
After this plan: the repo can *measure* whether a skill changes behavior —
saturated scenarios are mechanically distinguished from real nulls, at least two
judgment skills carry real ledger verdicts from seeded-defect probes, gates can
no longer be quietly loosened, and the always-loaded skill-context cost is
measurably reduced. All of it lands as gates, skills, and probe content — zero
new `ao` root commands (`cli/cmd/ao/default_spine_test.go` seals the spine).

## Ground truth (Extend row)

Repo patterns + two recovered/validated sources: the probe harness contract
(`scripts/probe-skill.sh`, `agentops-skill-probe.v1`, `evals/skill-probes/README.md`)
and the clean-room package at `recover/gstack-clean-room`
(`git show 9872483bd:skills/skill-eval/references/seeding.md` and
`:skills/skill-eval/scripts/saturation.sh`). Control experiment per lane: the
simplest change satisfying acceptance, with the insufficiency of doing less named
in the lane report. This is not integration-class work; the only external tool
touched (ee) is named in a docs row, never wired.

## Lanes and dependency order

Two merge trains. Train 1 lanes are mutually disjoint; Train 2 starts after
Train 1 merges (L2 consumes L1's gate; L3 appends to the gate registry file L1
owns in Train 1).

### Train 1

**L1 — measurement substrate** (M)
Re-land the clean-room package re-validated against current main, and port the
saturation rule to Go as a registered gate.
- Cherry-pick/adapt from `recover/gstack-clean-room`: `skills/skill-eval/`,
  `skills/route/`, `skills/one-way-door/`, the premortem reversibility check,
  the council `caller_challenge` protocol, `evals/skill-probes/RUNBOOK` additions.
  18 days of drift: rerun all four skill validators, fix contract drift, regen.
- New gate `skill.probe-headroom` in `cli/internal/gates/` (+ backing
  `scripts/check-skill-probe-headroom.sh` or pure-Go check per the repo's gate
  pattern): reads `agentops-skill-probe.v1` scorecards, classifies SATURATED
  (control arm already passes: no headroom, row is void) vs measured. RED first:
  a fixture scorecard pair where control==treatment==pass must be flagged; an
  INERT-with-headroom pair must not. `Blocking: false` (advisory-first; ratchet
  later on evidence, never by calendar — evidence-floor lesson).
- `check-skill-probe-coverage.sh` gains a **declared denominator**: an explicit,
  argued exclusion list (gbrain orphan-denominator pattern) so 0/N stops
  counting category errors as failures. Each exclusion carries its argument.
- `skills/skill-eval/SKILL.md` defers the saturation rule to the gate id; the
  recovered `saturation.sh` stays only as cited prior art in the lane report
  (no new shipped skill Python/sh logic — ratchet intent, not just letter).
- Write scope (class): `skills/{skill-eval,route,one-way-door,premortem,council}/**`
  + all outputs of `scripts/regen-all.sh` for those skills (skills-codex twins,
  catalog, SKILL-ROUTER, documentation-index) + matching `images/gemini/skills/`
  copies (byte-identity contract) · `cli/internal/gates/**` ·
  new `scripts/check-*.sh` + their bats · `evals/skill-probes/RUNBOOK*` ·
  `cli/cmd/ao` gate-wiring tests if counts are pinned (grep first).
- Acceptance: four skill validators green; `ao gate check --full` green; the
  RED fixture flips; recovered skills pass fresh validation against *current*
  main, not against their 08-08 base; no new root command (spine test unchanged).

**L4 — skill-context diet** (S+M)
- `disable-model-invocation` for human-only skills. Candidate set: the 35 skills
  carrying `user-invocable` metadata — but FIRST read the invocation graph
  (`ao skills consumers/graph`, plus grep of `workflows/*.js` and SKILL.md
  bodies): any skill invoked by another skill or workflow stays model-invoked.
  The frontmatter schema (`schemas/skill-frontmatter.v1.schema.json`,
  `additionalProperties: false`) must admit the key — schema edit + fixture +
  codex-projection handling are in scope (enumerate converter touchpoints before
  editing; if the Codex twin needs a policy field, that is part of this lane).
- One router skill (user-invoked) naming the human-only set and when to reach
  for each — it hints, never fires. Respect the 4 known reference-file gate
  footguns (owning SKILL.md link, codex twin prose, mkdocs allowlist,
  regen-changed-scope).
- Formalize `.out-of-scope/`: dated one-file-per-decision records; seed with the
  three decisions already made this week (CE solutions-store; ee
  outcome/curate/learn loops; whole-skill A/B as the measurement unit), each
  citing its evidence.
- Write scope (class): `skills/**` EXCEPT the skills owned by L1 and L2 + regen
  outputs + gemini copies of touched skills · `schemas/skill-frontmatter.v1.schema.json`
  + its fixture · converter code only if the key requires projection support ·
  `.out-of-scope/**` · `docs/` pages that document frontmatter keys.
- Acceptance: before/after byte+token count of always-loaded descriptions
  reported (deterministic — this lane needs no model eval); zero
  programmatically-invoked skills lost model-invocation (graph evidence in the
  report); validators + `regen-all.sh --check` green.
- Non-goal: no corpus-wide "It's working if" sweep, no leading-words rewrite,
  no negation-phrasing edits — measurement first (ce-retune Phase-0 lesson:
  no corpus-wide prose change lands ahead of the instrument that would grade it).

**L5 — consume-ee naming + retrieval-eval contract with a live consumer** (S/M)
- `AGENTS.md` federated-authority row: name ee as a concrete example of "a
  caller-selected memory system" (one row edit; run the AGENTS.md-sensitive
  gates: hookless-cold-start, doc-hooks-drift, and whatever `ao gate check
  --dry-run` routes for it).
- `schemas/pack-quality-expectations.v1.schema.json` — the fixture SHAPE from
  ee's eval (expected/forbidden ids, provenance density, token budget), adopted
  as the repo's retrieval-eval contract. Anti-ceremony demands a consumer at
  landing, so this lane also wires one: the existing, currently-unrun
  `evals/routing-probes/` get ≥3 hand-authored goldens in the new shape and a
  grader script run as an **advisory** job in `nightly.yml`. No store, no
  engine — deterministic grading of skill routing against declared goldens
  (the curation+measurement door ADR-0011 left open).
- Write scope: `AGENTS.md` (one row) · `schemas/pack-quality-expectations.v1.schema.json`
  · `evals/routing-probes/**` · one advisory job in `.github/workflows/nightly.yml`
  · the grader under `scripts/`.
- Acceptance: schema validates its goldens; grader runs green locally and is
  wired advisory in nightly; zero-goldens is a failing state for the grader
  (no new zero-denominator green — the goals lesson); AGENTS.md gates green.

### Train 2 (after Train 1 merges)

**L2 — seeded-defect probes for the judgment spine** (M)
- Four tier-2 scenarios per the recovered `seeding.md` doctrine, one each for
  `validate`, `premortem`, `council`, and one more chosen from the probe
  ledger's product/judgment list: a realistic sub-40-line work artifact with N
  planted defects the skill cannot honestly miss; floor assertion (≥1 action)
  and band assertion (count ∈ [N−1, N+2]); ~2 calibration reps per scenario
  before the live A/B; weaker-worker arms via the harness's existing
  `--model/--effort`.
- Each of the four skills gains a 3–4 line "It's working if" observable-tells
  block (checkable from the trace), which doubles as the independent judge's
  rubric — the judge is a separate fresh context, never the producing arm
  (gbrain SkillOpt: self-grade 1.00 vs independent 0.28 on a gamed skill).
- Every new ledger row must cite a PASSING `skill.probe-headroom` pre-screen.
- Write scope (class): `evals/skill-probes/**` · the four skills' SKILL.md
  tells-blocks + their regen outputs + gemini copies · probe fixtures.
- Acceptance: ≥2 skills move off UNMEASURED with headroom-verified verdicts —
  **either direction**; INERT-with-headroom is a successful measurement and an
  honest null, not a failure. Probes replay from immutable fixtures; ledger
  rows carry manifests.

**L3 — gate hardening** (S/M)
- `gate.tightening-ratchet`: a check over the changed diff that fails when a
  threshold/tolerance/suppression in `cli/internal/gates/**` or
  `scripts/check-*.sh` is LOOSENED without a `Gate-Loosen-Reason:` commit
  trailer (mirrors the proven `Test-Removal-Reason` mechanism). Tightening is
  always free. Advisory-first.
- `evidence.grounding`: deterministic pass over repo-tracked evidence documents
  (`docs/audits/**`, `docs/evidence/**`, handoffs) flagging cited paths that do
  not exist, full-length SHAs that do not resolve, and scaffold leaks
  (`{{…}}`, template headers). Mechanical pass ONLY — CE's semantic-validator
  half is explicitly out (that judgment already lives in the validate skill).
  Advisory-first; dated/HISTORICAL docs follow the existing docs-scope
  exemption, and flagged-but-legitimate historical citations get the same
  baseline treatment as the snippets gate.
- Write scope: `cli/internal/gates/**` (post-L1 merge) · new `scripts/check-*.sh`
  + bats · gate docs.
- Acceptance: RED first for both (a seeded loosening diff; a seeded dead-path
  evidence doc); both gates advisory; `--full` green on the real tree — any
  true findings the grounding gate surfaces on existing docs are fixed or
  baselined explicitly in the lane report, never silently.

## Non-goals (this plan)

No new `ao` root commands. No corpus-wide prose sweeps (tells/leading-words/
negation) — the negation-phrasing A/B is a *later experiment on top of* L1+L2's
substrate. No ee wiring beyond the AGENTS.md naming; no `ee init` in this repo.
No CE solutions-store or any checked-in knowledge corpus. No `evalsubstrate`
retirement, no `DeprecatedCommands` map-wide reconciliation, no release, no
promotion of any advisory gate to blocking (each promotes only on measured
evidence, in its own future change).

## Validation strategy

Per the repo contract: each lane implements in an isolated worktree, then a
fresh per-lane validator re-runs the acceptance itself against the exact branch
content. Cross-family (Codex) review of each integrated train before push.
One bounded repair round per review; a second non-PASS on the same intent is a
hard stop that returns to the caller. Merge trains land as two PRs (Train 1,
Train 2), auto-merge on green, branches updated onto main as needed.

## First useful check

The RED fixture for `skill.probe-headroom`: two committed scorecard pairs —
one saturated (control==treatment==pass), one INERT-with-headroom — and a
failing test asserting the gate separates them. Runnable before any other work.
