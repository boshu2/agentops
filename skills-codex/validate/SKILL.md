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
- Consume exact-input deterministic receipts after verifying candidate/tree,
  command/mode, registry/toolchain, and environment identity. Rerun only
  missing, stale, suspicious, or invalidated facts; an unchanged broad receipt
  is proof, not an invitation to pay for the same suite again.
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
- Validate one frozen bounded tranche, never each intermediate Crank wave. One
  complete review may be followed by one affected-claim closure after the
  producer's consolidated repair; a second distinct repair need is `REPLAN`.
- The repository's full deterministic terminal gate runs once on the final
  post-repair candidate. Its exact-input receipt is part of the sealed verdict
  and is reusable by delivery; it is not rerun at every earlier boundary.
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

## Fresh-context dispatch boundary

A schema-valid factual `READY` receipt, frozen candidate, explicit acceptance,
and distinct author/judge identities permit `VALIDATE_SINGLE_FRESH`. Validate does not meter, reserve, or authorize semantic work through a second adapter.

A mandatory factual `FAIL`, any `ERROR` or `UNKNOWN`, or missing or malformed
proof stops before judge spend and returns the factual evidence to the caller.
Diagnostic and release `FAIL` remain nonbinding under the factual receipt
contract. Runtime time, cost, or quota limits are external facts; Validate
reports them without creating counters, helper state, or escalation authority.

## Modes

| Mode | Judge shape | Purpose |
|---|---|---|
| default | one independent judge | general evidence-bound verdict |
| `--quick` | one fresh independent judge, narrow claims | bounded evidence-bound verdict |
| `--deep` | up to four independent perspectives | high-risk completeness |
| `--mixed` | explicitly selected model families | cross-family review |
| `--debate` | two rounds | contested judgment |
| `--mode=post-impl` | acceptance plus completion checks | implemented work |
| `--mode=pre-impl [--target=X]` | plan/spec checks | planned work |
| `--mode=pr` | diff plus acceptance checks | submission artifact |

**Mode-budget assertion:** 8 modes. Adding a ninth requires merging or removing
an existing mode. The folded `vibe` trigger maps to `--mode=post-impl`.

## Workflow

1. **Pin tranche and acceptance.** Record artifact path, exact tranche
   commit/digest, author
   identity, mode, required checks, and declared coverage exclusions.
2. **Verify deterministic receipts.** Reuse each exact-input receipt whose
   identities still match. Execute only missing, stale, suspicious, or
   invalidated commands. A red mandatory command stops judge spend and is
   attributed against the exact base before any REPAIR handoff.
3. **Run fresh-context judgment once.** Give the judge only the pinned artifact,
   acceptance contract, verified factual receipts, standards, and output path.
   The judge verifies evidence identity and reruns a command only when its
   receipt is invalid or the semantic claim makes it suspicious.
4. **Consolidate fail-closed.** PASS needs complete proof. WARN discloses a
   nonblocking concern. FAIL records any blocker, stale artifact, counterfeit
   independence, malformed evidence, or mandatory red check.
5. **Seal final deterministic proof.** After any consolidated repair and
   affected-claim closure, consume one full terminal-gate receipt for the final
   exact candidate. Missing or red terminal proof is FAIL.
6. **Write one immutable output.** Emit canonical `result.json`. If a caller
   still requires Markdown, generate a concise link-only projection from that
   JSON; do not author a second analysis. Each structured observation contains
   `kind`, `summary`, and `evidence_ref`.
7. **Return proof to the caller.** Report verdict, findings, observations,
   `not_checked`, artifact identity, and one suggested owner/action. Stop.

Detailed mode and evidence rules live in
[canonical-validation-protocol.md](references/canonical-validation-protocol.md).
The proof-only post-verdict boundary is in
[post-verdict-actions.md](references/post-verdict-actions.md). Quick mode is
defined in [quick-mode-vibe.md](references/quick-mode-vibe.md).

## Output Specification

- **Artifact directory:** invocation output root for canonical `result.json`;
  optional generated Markdown may live under `.agents/council/`.
- **Filename convention:** `result.json`; optional
  `YYYY-MM-DD-validate-<topic>.md` is a projection only.
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
- [ ] Every mandatory fact has an exact-input receipt; only missing, stale,
      suspicious, or invalidated commands were rerun.
- [ ] Factual proof is READY and author/judge identities differ before fresh
      semantic judgment begins.
- [ ] Independent PASS has different author and judge identities.
- [ ] Findings, observations, and coverage gaps cite evidence.
- [ ] Canonical `result.json` is valid; any optional Markdown projection cites
      it and adds no independent verdict.
- [ ] No implementation, learning, retry, tracker, or delivery action occurred.

Executable behavior is in [validate.feature](references/validate.feature).
