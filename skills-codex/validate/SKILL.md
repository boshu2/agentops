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
2. **Run deterministic checks.** Execute the smallest commands that directly
   prove the acceptance examples. A red mandatory command is a FAIL; do not
   spend judge effort rediscovering it.
3. **Run fresh-context judgment.** Give the judge only the pinned artifact,
   acceptance contract, required commands, standards, and output path. The
   judge reruns evidence rather than trusting producer summaries.
4. **Consolidate fail-closed.** PASS needs complete proof. WARN discloses a
   nonblocking concern. FAIL records any blocker, stale artifact, counterfeit
   independence, malformed evidence, or mandatory red check.
5. **Write immutable outputs.** Emit `result.json` and one markdown verdict.
   Each structured observation contains `kind`, `summary`, and `evidence_ref`.
6. **Return proof to the caller.** Report verdict, findings, observations,
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
- [ ] Mandatory commands were rerun by the validator.
- [ ] Independent PASS has different author and judge identities.
- [ ] Findings, observations, and coverage gaps cite evidence.
- [ ] Machine and markdown verdicts agree.
- [ ] No implementation, learning, retry, tracker, or delivery action occurred.

Executable behavior is in [validate.feature](references/validate.feature).
