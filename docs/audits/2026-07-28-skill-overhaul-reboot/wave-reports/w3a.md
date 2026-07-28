# Wave W3a — evidence/judgment skills (first half)

Bead: `age-skill-overhaul-reboot-sjv7v.4` (first half). Branch: `claude/sor-w3a-judgment`.
Skills in scope (6): **council, premortem, postmortem, scope, reality-check, idea-genie**.

Method: verify every checklist item against the live tree → fix confirmed `[defect]`s →
apply `[enhance]` tightenings or defer/reject with a one-line reason. No silent drops.

## Decisive verification finding (governs several items)

**This reboot tree is on `verdict.v2`.** There is no `validate_v3.py`, no `kernel_v3.py`,
no `verdict.v3` / `subject-manifest.v2` / `scope-index.v1` schema anywhere. `skills/validate`
produces `verdict.v2` (`schemas/verdict.v2.schema.json`; SKILL.md line 61: "do not change
`verdict.v2` schema"). The checklist's cross-cutting "the executable loop emits `verdict.v3`
via `validate_v3.py`" claim describes the abandoned `codex/skill-overhaul-20260724` branch,
**not** this tree. Consequence: a skill that *consumes* verdict evidence correctly names
`verdict.v2` here; only a skill that must never *mint* a verdict gets its denial broadened.

## Disposition

### council
- **[defect] dangling `council-report.v1` (zero schema hits)** — FIXED. Added
  `skills/council/schemas/council-report.v1.schema.json` (closed-key object: question,
  subject_digest, ≥2 judges each with context_id/methodology/judgment/evidence, and a
  consensus/divergence/minority/unresolved synthesis) + `scripts/validate-output.sh` (jq).
  `output_contract` now names the validator.
- **[defect] v2 vocabulary (SKILL.md:78)** — FIXED. "does not write `verdict.v2`" →
  "does not mint a verdict of any version — no `PASS`/`FAIL`/`NOT_PROVEN`, no `verdict.v*`".
- **[enhance] no `scripts/validate.sh`, no feature file — give a schema+validator** — APPLIED
  (schema + validate-output.sh above) and added `scripts/validate.sh` contract guard (pins the
  boundary line; strip-tested: fails when the pin is removed). Feature file not added — the
  schema+validator satisfies the contract need; an optional BDD projection is out of scope.
- (tightening) added a negative trigger (do not convene for a routine/reversible decision) and
  a failure line (a timed-out / evidence-free judge is excluded and counted as non-returning;
  <2 independent judgments → report the round insufficient, not a thin consensus). Added an
  Output section with the artifact dir under `.agents/scratch/council/` (ADR-0016 closed set).

### premortem
- **[defect] v2 vocabulary (SKILL.md:86) "not `verdict.v2`"** — FIXED →
  "no verdict of any version". Retains the `advisory findings` phrase its validator pins.
- **[enhance] no scenario coverage (fail 0/2)** — REJECTED (stale). `references/premortem.feature`
  already carries 2 scenarios ("A fresh judge returns advisory findings", "Premortem stops
  after the review"). The 0/2 count is from the seed-era tree.

### postmortem
- **[defect] `output_contract` points at a `.feature` file** — FIXED. `output_contract` now
  describes the actual markdown artifact (`postmortem-report.md`, matching `produces`); the
  `.feature` remains linked from the body as executable behavior, which is its real role.
- **[defect] v2 vocabulary: `consumes: [verdict.v2]`** — REJECTED (stale). This tree emits
  `verdict.v2`; postmortem correctly consumes the real artifact. Changing to `verdict.v3`
  would point at a schema that does not exist here.
- **[enhance] `effects: []` while it writes `postmortem-report.md`** — APPLIED.
  `effects: [write_postmortem_report]` (matches its sibling `write_advisory_*` declarations).
- **[enhance] artifact dir `.agents/postmortem/` outside ADR-0016 closed set** — APPLIED.
  Moved to `.agents/scratch/postmortem/` (ADR-0016 §1: closed set = `ao/`, `scratch/`,
  `projections/`; convention `scratch/WRITER/…`).
- **[possibly-landed] `fm-ws-noncanonical-topdir` detector absent from `cli/`/`scripts/`** —
  DEFERRED. Verified still absent, but it is an `ao doctor` (Go CLI) concern, not skill text;
  out of scope for a skill-text wave. The skill-side fix is the dir move above.
- **[enhance] no scenario coverage (fail 0/2)** — REJECTED (stale). `references/postmortem.feature`
  already carries 2 scenarios.

### scope
- **[enhance] YAML output block is prose-only — add an output validator** — REJECTED. scope is
  response-only (`effects: []`, `output_contract: 'response: …'`); it persists no artifact, so
  there is nothing on disk to validate. An output validator would require inventing a persisted
  artifact contract (new `produces`/`effects`), changing the skill's declared nature — beyond a
  text-enhancement wave. (scope already has a `scripts/validate.sh` contract guard, contrary to
  the checklist's stale "no validator" premise.)
- **[enhance] no feature file** — DEFERRED. Optional BDD projection; scope's contract is already
  pinned by its `validate.sh` (heal `--check --strict` + forbidden-authority grep). Adding one
  is a low-value projection with no defect behind it.
- No file change to scope: it has zero `[defect]`s and an already-tight contract (distinct
  intent, supplier-to-plan placement, advisory boundary, explicit failure-behavior section).

### reality-check
- **[defect] dangling `reality-check-report.v1` (zero schema hits)** — FIXED. Added
  `skills/reality-check/schemas/reality-check-report.v1.schema.json` (closed-key object: claim,
  findings each categorized confirmed/gap/incomplete-evidence/changed-assumption with evidence,
  optional goal-coverage dispositions) + `scripts/validate-output.sh` (jq). `output_contract`
  now names the validator.
- **[enhance] one tracked file, no `validate.sh` — give a schema+validator** — APPLIED
  (schema + validate-output.sh above) and added `scripts/validate.sh` contract guard
  (strip-tested: fails when the boundary pin is removed).
- (tightening) added a "no verdict/`PASS` of any version" boundary and a failure line (an
  untestable claim is reported incomplete-evidence with the missing artifact named, never
  resolved as confirmed). Added an Output section with the artifact dir under
  `.agents/scratch/reality-check/`.

### idea-genie
- **[enhance] artifact dir `.agents/ideas/<run-id>/` violates ADR-0016 closed set** — APPLIED.
  Moved to `.agents/scratch/ideas/<run-id>/`; updated `tests/scripts/agentops-native-skills.bats`
  fixtures (`.agents/ideas/run-1|2` → `.agents/scratch/ideas/run-1|2`). Validator logic unchanged
  (it only asserts `handoff.artifact_dir` is a nonempty string).
- **[enhance] add a shared context-isolation reference linked from both council and idea-genie**
  — DEFERRED. Introduces a new shared reference artifact whose canonical home is contested — the
  `shared` skill is slated for semantic retirement per this reboot plan's structural beads — and
  both skills already specify their isolation requirements inline (distinct context IDs, sealed
  generation, fresh sessions per round). Defer to a dedicated follow-up once `shared`'s
  disposition is settled.

## Cross-cutting item (not a W3a skill)
- The checklist's cross-cutting `[defect]` (AGENTS.md / operating-loop.md v1/v2 vocabulary vs a
  claimed v3 executable) is **not** a W3a skill file and is moot here: this tree is on v2 (see
  the verification finding). Any doctrine-doc rewrite belongs to the kernel wave (W1), not W3a.

## Disposition counts (17 checklist items total)
- Fixed (defect): 5 — council×2, premortem×1, postmortem×1 (output_contract), reality-check×1.
- Applied (enhance): 5 — council validator/guard, postmortem effects, postmortem dir,
  reality-check validator/guard, idea-genie dir+fixtures.
- Rejected (stale/structural, with reason): 4 — postmortem `consumes: [verdict.v2]` (defect;
  tree is v2), premortem scenario coverage, postmortem scenario coverage, scope output-validator.
- Deferred (with reason): 3 — scope feature file, postmortem `fm-ws-noncanonical-topdir`
  detector (CLI concern), idea-genie shared context-isolation reference.

## Gate results (all green)
- `bash skills/{council,reality-check,premortem,postmortem,scope}/scripts/validate.sh` — PASS (5/5).
- Output validators smoke-tested: valid artifacts pass; smuggled `verdict`/`readiness` field,
  single-judge council, bad category, and empty findings all rejected.
- Contract guards strip-tested: council & reality-check `validate.sh` fail when the pinned
  boundary line is removed; no inert `!`-negation in any new script.
- `bash scripts/validate-skill-frontmatter.sh` (non-strict) — 49/49 ok.
- `bash scripts/regen-all.sh` then `--check` — all generated projections current.
- `bats skill-validator-liveness.bats anti-spiral-contract.bats agentops-native-skills.bats` — 23/23.
- `bash tests/skills/test-token-budgets.sh` — 0 failures (descriptions ≤180; codex avg 39/45).
- `bash scripts/check-skill-python-ratchet.sh` — PASS; no new `.py` under `skills/`.
