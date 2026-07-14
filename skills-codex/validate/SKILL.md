---
name: validate
description: Independently remeasure a bounded artifact
---
# Validate

> **Purpose:** Independently remeasure one bounded artifact against explicit
> acceptance and emit immutable proof. Validate ends at proof.

## Critical Constraints

- **One role: validator.** Because independence is the proof boundary, never
  edit the subject, control its producer, mutate repository or tracker state,
  or take delivery authority.
- Pin the artifact by path plus commit or digest before checking it because a
  changed artifact makes prior evidence stale.
- Rerun cited deterministic commands on the pinned artifact because author
  claims and conversational memory are context, not proof.
- Because claimed independence must be real, PASS requires every mandatory
  check green, no blocker, disclosed `not_checked`, and a judge identity
  different from the author.
- Judge lanes are read-only except for their one verdict artifact.
- Structured observations are part of the immutable verdict; they describe
  evidence without classifying recurrence, promoting knowledge, or changing
  future work.
- WARN and FAIL identify the owning producer and an executable next action, but
  Validate does not perform the action, retry, re-plan, or choose escalation.
- Use runtime-native fresh context. Additional judges are optional depth, not a
  substitute for one accountable validator.
- When a repeated FAIL trips a breaker, the orchestrator does not page the
  operator first: it dispatches exactly one bounded fresh-context (or
  cross-family) **helper** pass. HELPER-UNSTUCK means the helper cleared the
  blocker and the proof is resumed on an explicit orchestrator decision;
  HELPER-ESCALATE reaches a human only when that single helper pass also fails
  (or the class is a refusal/judgment/spent-ceiling skip).

## Frozen request boundary

Before any judge or model spend, freeze the explicit base, candidate commit and
tree, a nonempty subtree set, the complete changed-surface set as owned blobs or
deletions, acceptance, resolvable claim references plus their digests, evidence,
factual-gate registry, toolchain, author, and validator route. Never infer the
base from the current branch or an upstream name, and never accept an omitted or
extra owned path.

Atomically reserve a canonical request identity, keyed by request ID plus the
canonical-JSON request digest in the Git common directory, before reserving the caller's
receipt path and before the first factual gate. The same request cannot run
again by choosing another output. Run each selected gate at most once at the
candidate in its declared JSON mode. A concurrent caller or stale reservation
refuses and stays in HOLD; there is no automatic crash recovery. A stale or
mutated candidate, missing/stale claim or evidence, invalid registry entry,
missing registry backing, ERROR, UNKNOWN, or mandatory FAIL stops before judge spend. A FAIL is eligible for
REPAIR only after the same gate is rerun at the frozen exact base and passes
there; a failure at both commits is pre-existing evidence returned to the
caller. A diagnostic or release FAIL remains nonbinding to semantic validation,
though it retains baseline attribution. Green mandatory proof routes to one
fresh validator by default. Inventory size is never a rigor or validator-count
signal.

Every factual registry entry declares one closed `proof_kind`: syntax, schema,
identity, paths, generated drift, executable assertion, or evidence integrity.
Semantic prose scores and exact-wording preferences are reviewer evidence, not
factual gate kinds. Missing backing is a typed `registry_integrity` defect;
neither that defect nor a semantic observation may be mislabeled as candidate
proof.

The portable freezer, runner, and receipt verifier is
`python3 skills/validate/scripts/validation-request.py freeze|run|check-receipt --help`.

## Run-budget admission

A factual `READY` receipt is necessary but not sufficient to dispatch the fresh
validator. Before dispatch, Validate must consume one `semantic-review`
admission from RPI's existing persistent governor with the same run ID and all
four explicit meters. The adapter is:

`python3 skills/validate/scripts/validation-budget.py admit|check-receipt --help`.

Only its schema-valid `AUTHORIZED` receipt permits `VALIDATE_SINGLE_FRESH`.
A mandatory factual `FAIL`, any `ERROR` or `UNKNOWN`, missing or malformed
factual proof, a missing meter, missing/corrupt run state, or governor refusal
produces `NONAUTHORIZING` evidence. Diagnostic and release `FAIL` remain
nonbinding under the factual receipt contract; when S1/S8 emit an aggregate
`READY` receipt, the budget adapter does not reclassify its lanes. A hard-ceiling
refusal preserves the governor's typed `ANDON` reason and
`helper_allowed:false`; Validate does not purchase recovery after the ceiling.
The adapter writes proof of the governor admission and recorded charge, not
another durable controller. It has no local attempt, retry, phase-budget,
helper, or escalation state.

## Modes

| Mode | Judge shape | Purpose |
|---|---|---|
| default | one independent judge | general evidence-bound verdict |
| `--quick` | inline, independence waived | bounded sanity check |
| `--deep` | up to four independent perspectives | high-risk completeness |
| `--mixed` | explicitly selected model families | cross-family review |
| `--debate` | two rounds | contested judgment |
| `--mode=post-impl` | acceptance plus completion checks | implemented work |
| `--mode=pre-impl [--target=X]` | plan/spec checks | planned work |
| `--mode=pr` | diff plus acceptance checks | submission artifact |

**Mode-budget assertion:** 8 modes. Adding a ninth requires merging or removing
an existing mode. The folded `vibe` trigger maps to `--mode=post-impl`.

## Workflow

1. **Pin subject and acceptance.** Record artifact path, commit/digest, author
   identity, mode, required checks, and declared coverage exclusions.
2. **Run deterministic checks.** Execute the frozen selected commands that
   directly prove the acceptance examples. A red mandatory command stops judge
   spend and is attributed against the exact base before any REPAIR handoff.
3. **Admit semantic review.** Bind the factual receipt to the persistent run,
   submit all four measured charges to the sole governor, and require a durable
   `AUTHORIZED` receipt before dispatch.
4. **Run fresh-context judgment.** Give the judge only the pinned artifact,
   acceptance contract, required commands, standards, and output path. The
   judge reruns evidence rather than trusting producer summaries.
5. **Consolidate fail-closed.** PASS needs complete proof. WARN discloses a
   nonblocking concern. FAIL records any blocker, stale artifact, counterfeit
   independence, malformed evidence, or mandatory red check.
6. **Write immutable outputs.** Emit `result.json` and one markdown verdict.
   Each structured observation contains `kind`, `summary`, and `evidence_ref`.
7. **Return proof to the caller.** Report verdict, findings, observations,
   `not_checked`, artifact identity, and one suggested owner/action. Stop.

Detailed mode and evidence rules live in
[canonical-validation-protocol.md](references/canonical-validation-protocol.md).
The proof-only post-verdict boundary is in
[post-verdict-actions.md](references/post-verdict-actions.md). Quick mode is
defined in [quick-mode-vibe.md](references/quick-mode-vibe.md).

## Output Specification

- **Artifact directory:** `.agents/council/` for markdown; invocation output
  root for `result.json`.
- **Filename convention:** `YYYY-MM-DD-validate-<topic>.md` and `result.json`.
- **Serialization:** `result.json` follows
  [`schemas/verdict.v1.schema.json`](../../schemas/verdict.v1.schema.json).
- **Evidence:** exactly one anchored `VERDICT: PASS|WARN|FAIL`, a nonempty
  `COMMANDS RUN:` section with `judge=<id> command=<command>`, `REASONS:`,
  findings, structured observations, and `not_checked`.
- **Validator command:** `bash skills/validate/scripts/validate.sh`.
- **Downstream handoff:** callers may pass the immutable verdict and digest to
  Learn or to their own delivery process. A repository may consume PASS without
  another LLM landing verdict. Validate has no authority after the handoff.

## Quality Checklist

- [ ] Subject identity and acceptance are pinned.
- [ ] Base, candidate/tree, subtrees, changed surfaces, dependencies, registry,
      toolchain, author, and validator route are frozen and still match.
- [ ] Mandatory commands were rerun by the validator.
- [ ] The same run durably recorded one semantic-review admission and all four
      charges before validator dispatch.
- [ ] Independent PASS has different author and judge identities.
- [ ] Findings, observations, and coverage gaps cite evidence.
- [ ] Machine and markdown verdicts agree.
- [ ] No implementation, learning, retry, tracker, or delivery action occurred.

Executable behavior is in [validate.feature](references/validate.feature).
