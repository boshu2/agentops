---
name: validate
description: Independently remeasure a bounded artifact
---
# Validate

> **Purpose:** Independently remeasure one bounded artifact against explicit
> acceptance and emit immutable proof. Validate ends at proof.

## Critical Constraints

- **Why: preserve independence. One role: validator.** Never edit the subject, control its producer, mutate
  repository or tracker state, or take delivery authority.
- **Why: prevent stale proof.** Pin the artifact by path plus commit or digest before checking it. A changed
  artifact makes prior evidence stale.
- **Why: ground the verdict.** Rerun cited deterministic commands on the pinned artifact. Author claims and
  conversational memory are context, not proof.
- **Why: fail closed.** PASS requires every mandatory check green, no blocking finding, disclosed
  `not_checked`, and a judge identity different from the author when
  independence is claimed.
- **Why: protect the subject.** Judge lanes are read-only except for their one verdict artifact.
- **Why: keep evidence reusable.** Structured observations are part of the immutable verdict; they describe
  evidence without classifying recurrence, promoting knowledge, or changing
  future work.
- **Why: preserve ownership.** WARN and FAIL identify the owning producer and an executable next action, but
  Validate does not perform the action, retry, re-plan, or choose escalation.
- **Why: reduce shared-context bias.** Use runtime-native fresh context. Additional judges are optional depth, not a
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

### Compatibility references

The active workflow above does not load the older specialist material below by
default. These paths remain discoverable because live skills and distribution
images still cite them; when selected explicitly, the canonical protocol and
constraints above take precedence:

[complexity-analysis.md](references/complexity-analysis.md),
[deep-audit-protocol.md](references/deep-audit-protocol.md),
[deep-checks.md](references/deep-checks.md), [examples.md](references/examples.md),
[go-patterns.md](references/go-patterns.md),
[go-standards.md](references/go-standards.md),
[json-standards.md](references/json-standards.md),
[markdown-standards.md](references/markdown-standards.md),
[patterns.md](references/patterns.md),
[python-standards.md](references/python-standards.md),
[report-format.md](references/report-format.md),
[rust-standards.md](references/rust-standards.md),
[shell-standards.md](references/shell-standards.md),
[test-pyramid-inventory.md](references/test-pyramid-inventory.md),
[test-pyramid-weighting.md](references/test-pyramid-weighting.md),
[typescript-standards.md](references/typescript-standards.md),
[verification-report.md](references/verification-report.md),
[vibe-coding.md](references/vibe-coding.md),
[vibe-suppressions.md](references/vibe-suppressions.md),
[vibe.feature](references/vibe.feature),
[write-time-quality.md](references/write-time-quality.md), and
[yaml-standards.md](references/yaml-standards.md).

For entry-documentation work, load
[`docs/contracts/entry-documentation-behavior.md`](../../docs/contracts/entry-documentation-behavior.md)
and judge the pinned four-document journey. Prose meaning is a judgment surface:
deterministic prechecks may verify identity, links, command existence, and
verdict shape, but never replace the judges with keyword or regex semantics.

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
