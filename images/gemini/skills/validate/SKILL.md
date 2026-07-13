---
name: validate
spine: true
description: 'Produce PASS/WARN/FAIL verdicts for artifacts, plans, code, PRs, or gates — quick pre-commit checks (absorbs vibe) through completion audits. Triggers: "validate an artifact", "PASS/WARN/FAIL verdict", "readiness / completion audit".'
practices:
- design-by-contract
- llm-eval-harness
hexagonal_role: driving-adapter
consumes: []
produces:
- result.json
context_rel: []
skill_api_version: 1
user-invocable: true
context:
  window: fork
  intent:
    mode: task
  sections:
    exclude:
    - HISTORY
  intel_scope: full
metadata:
  graph_root: true
  tier: judgment
  dependencies: [pawl-review]
output_contract: schemas/verdict.v1.schema.json
---
# /validate — Canonical Validator

> **Purpose:** Move 6 of the operating loop: independently remeasure a bounded artifact or slice, prove its acceptance, and emit one schema-valid PASS/WARN/FAIL verdict with an executable next action.

## Critical Constraints

- **One role: validator.** Never edit the artifact, commit, push, close tracker work, or operate infrastructure. **Why:** a judge that mutates the subject destroys independence.
- Rerun cited commands on the actual artifact; never accept author evidence, conversational memory, or uncited figures as proof. **Why:** validation remeasures rather than agrees.
- PASS requires every mandatory check green, all blockers resolved, disclosed `not_checked` coverage, and an independent judge when the mode claims independence. **Why:** missing evidence cannot be laundered into confidence.
- `judge_id == author_id` cannot independently PASS; inline `--quick` fallback is stamped waived. **Why:** self-grading is guidance, not assurance.
- Every judge brief says: **"READ-ONLY except writing your single verdict file at `<path>`. Do NOT commit, push, or run tracker/infra ops (git push, br/bd, dolt)."** **Why:** role scope is model-independent.
- This skill validates work; `ao pawl` certifies landing with a commit-bound verdict. **Why:** a pre-work PASS never pre-authorizes a main write.
- Use runtime-native judges for a requested multi-judge mode; never start NTM, Agent Mail, managed agents, Gas City, or another runtime unless explicitly selected. **Why:** judgment does not silently broaden orchestration.
- `WARN|FAIL|REFUTED -> AUTO-REDO`: consult the pawl, return findings as re-plan evidence, repair through the owning producer, and rerun the same checks. **Why:** ordinary negative verdicts are self-correction, not human andons.
- `BREAKER -> HOLD -> ONE-HELPER`; `HELPER-UNSTUCK -> AUTO-REDO`. Hold judgment and use one bounded local-shell helper to inspect contradictory evidence or a broken validator. **Why:** one recovery pass distinguishes tooling failure from a real stop.
- `HELPER-ESCALATE -> HUMAN`; `REFUSAL-LANE|EXPLICIT-JUDGMENT|EXHAUSTED-BUDGET -> HUMAN`. **Why:** waivers, risk acceptance, unavailable authority, refusal, or exhausted recovery require the operator.

## Modes

| Mode | Judge shape | Purpose |
|---|---|---|
| default | 2 independent | general artifact consensus |
| `--quick` | inline, waived | bounded sanity/readiness check |
| `--deep` | 4 perspectives | thorough requirement/feasibility/scope review |
| `--mixed` | explicitly selected families | cross-family review |
| `--debate` | 2+ judges, 2 rounds | adversarial critique/rebuttal |
| `--mode=post-impl` | pipeline + isolated judges | acceptance and completion audit |
| `--mode=pre-impl [--target=X]` | 2–4 judges | plan/spec/fitness/scope/skill/health audit |
| `--mode=pr` | diff + acceptance judges | submission readiness |

**Mode-budget assertion:** 8 modes. Adding a ninth requires merging or removing an existing mode.

Folded triggers remain load-bearing: **`vibe` → `--mode=post-impl`** for code
readiness, and `bead-completion-audit` routes to post-implementation closeout.
Detailed target and evidence rules live in
[canonical-validation-protocol](references/canonical-validation-protocol.md).

## Quick Start

```bash
/validate path/to/artifact
/validate --quick path/to/artifact
/validate --deep path/to/spec.md
/validate --mode=pre-impl --target=skill skills/example
/validate --mode=post-impl recent
/validate --mode=pr 123
```

## Execution Workflow

### 1. Resolve mode, artifact, and authority

Reject invalid mode/target combinations. Confirm the artifact exists, is within
scope, and has explicit acceptance. For a goal-design packet, run:

```bash
scripts/check-goal-design-packet.sh <packet-dir>
```

Nonzero is deterministic FAIL evidence. Load only the selected section of
[canonical-validation-protocol](references/canonical-validation-protocol.md)
plus the relevant language/risk standards.

**Checkpoint:** record artifact identity, commit/digest, mode, targets, author id,
required checks, and `not_checked` before dispatch.

### 2. Run deterministic prechecks

- pre-implementation: target rubric, temporal/error-rescue checks, test pyramid;
- post-implementation: complexity, bug sweep, slice acceptance, completion kernel;
- PR: upstream alignment first, contribution rules, atomicity, scope, tests/lint;
- all modes: source precedence, artifact freshness, and output-schema availability.

A deterministic red result is a FAIL verdict; do not spend judge tokens to
rediscover it.

### 3. Dispatch isolated judges

Register dispatch intent before spawning. Use the current runtime's native
subagents for multi-judge modes; if unavailable, degrade only to explicit
`--quick` and stamp the independence waiver.

Each judge receives the artifact path/digest, bounded context, mandatory checks,
verdict path, and exact read-only clamp. It independently reruns commands.

**Checkpoint:** author and judge identities differ, dispatch scopes do not
collide, and every claimed judge produced a nonempty verdict artifact.

### 4. Consolidate fail-closed

- PASS: all required judges/checks PASS (or the declared deep-mode majority), no blocker.
- WARN: nonblocking concern or explicit coverage gap remains.
- FAIL: any deterministic failure, blocker, counterfeit/self judge, stale artifact, or malformed evidence.

Contradictory verdicts are FAIL until reconciled; never average away a blocker.

### 5. Write and validate outputs

Write `.agents/council/YYYY-MM-DD-validate-<slug>.md` with exactly one anchored
`## Council Verdict: PASS|WARN|FAIL`, plus `result.json` matching
`schemas/verdict.v1.schema.json`. The report contains exactly one anchored
`VERDICT:`, a nonempty `COMMANDS RUN:`, `REASONS:`, findings, and `not_checked`.

**Checkpoint:** run both validators in Output Specification; PASS is illegal if
either output is missing, malformed, stale, self-graded, or unsupported by commands.

### 6. Route the verdict

Report verdict, key findings, artifact paths, and one executable next action.
WARN/FAIL re-enters the operating loop through the owning producer. Reusable
failures compile into a planning/pre-mortem check instead of becoming a dead log.

## Output Specification

**Artifact directory:** markdown verdicts under `.agents/council/`; machine-readable `result.json` at the invocation output root.

**Filename convention:** `.agents/council/YYYY-MM-DD-validate-<topic-slug>.md` plus `result.json`.

**Serialization/schema format:** `result.json` follows closed `schemas/verdict.v1.schema.json`; markdown has one `## Council Verdict: PASS|WARN|FAIL` and the anchored evidence form in the protocol reference.

**Validator command:** validate `result.json` with `Draft202012Validator(schema, format_checker=FormatChecker())`, an explicit timezone-aware RFC3339 parse of `validated_at`, and the pinned author identity so a PASS requires `validator_session != author_session`; then verify the markdown has exactly one council verdict, exactly one matching `VERDICT:`, and a `judge=<validator_session> command=<command>` evidence line under `COMMANDS RUN:` before `REASONS:`. The executable fixture suite is `bash skills/validate/scripts/validate.sh`.

**Downstream handoff:** pass mode/target, artifact path/digest/commit, author and judge identities/families, deterministic commands and exit codes, verdict/report paths, findings, `not_checked`, waiver/breaker state, and the exact next action; landing still requires `ao pawl`.

## Quality Checklist

- [ ] Artifact identity and acceptance are pinned before checks run.
- [ ] Judges are read-only, independent when claimed, and rerun commands themselves.
- [ ] Every number, timing, and commit is captured from cited output.
- [ ] PASS has no blockers, counterfeit judges, stale evidence, or undisclosed gaps.
- [ ] Markdown and JSON agree and pass their deterministic validators.
- [ ] WARN/FAIL/REFUTED returns to AUTO-REDO; only real breakers reach a human.
- [ ] Validation PASS is never substituted for the commit-bound landing verdict.

## Examples

### Reject a counterfeit completion claim

**User says:** `/validate --mode=post-impl recent`

**What happens:** the isolated judge reruns the slice acceptance command and
finds the author's reported count was inferred and the test exits nonzero.

**Result:** FAIL with captured output, corrected count, owning producer, and the
same validation command as next action; no human andon and no landing claim.

## Troubleshooting

| Problem | Response |
|---|---|
| Judge unavailable | Use explicit waived `--quick` or HOLD only if independence is required |
| Artifact changed mid-review | Mark stale, pin the new digest, rerun all checks |
| Judges disagree | FAIL until the blocker is reproduced and reconciled |
| Empty `COMMANDS RUN:` | Discard the verdict and dispatch a real verifier |

## Reference Documents

- [references/canonical-validation-protocol.md](references/canonical-validation-protocol.md) — modes, targets, evidence, isolation, landing boundary
- [references/report-format.md](references/report-format.md) — markdown detail
- [references/deep-audit-protocol.md](references/deep-audit-protocol.md) — deep-mode perspectives
- [references/test-pyramid-inventory.md](references/test-pyramid-inventory.md) — coverage selection
- [references/post-verdict-actions.md](references/post-verdict-actions.md) — bounded follow-through
- [references/validate.feature](references/validate.feature) — executable behavior
- [references/complexity-analysis.md](references/complexity-analysis.md)
- [references/deep-checks.md](references/deep-checks.md)
- [references/examples.md](references/examples.md)
- [references/go-patterns.md](references/go-patterns.md)
- [references/go-standards.md](references/go-standards.md)
- [references/json-standards.md](references/json-standards.md)
- [references/markdown-standards.md](references/markdown-standards.md)
- [references/patterns.md](references/patterns.md)
- [references/python-standards.md](references/python-standards.md)
- [references/quick-mode-vibe.md](references/quick-mode-vibe.md)
- [references/rust-standards.md](references/rust-standards.md)
- [references/shell-standards.md](references/shell-standards.md)
- [references/test-pyramid-weighting.md](references/test-pyramid-weighting.md)
- [references/typescript-standards.md](references/typescript-standards.md)
- [references/verification-report.md](references/verification-report.md)
- [references/vibe-coding.md](references/vibe-coding.md)
- [references/vibe-suppressions.md](references/vibe-suppressions.md)
- [references/write-time-quality.md](references/write-time-quality.md)
- [references/yaml-standards.md](references/yaml-standards.md)
- [references/vibe.feature](references/vibe.feature)
