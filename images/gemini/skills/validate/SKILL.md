---
name: validate
description: 'Freshly judge a finished change against its acceptance: PASS, FAIL, or NOT_PROVEN. Not for claim-vs-tree checks; that is reality-check. Triggers: "validate", "is this proven", "check this change".'
practices:
- design-by-contract
- llm-eval-harness
- content-addressed-storage
hexagonal_role: driving-adapter
consumes:
- subject-manifest.v1
produces:
- subject-manifest.v1
- validation-result
- verdict.v2
context_rel:
- kind: customer-of
  with: plan
- kind: customer-of
  with: implement
skill_api_version: 1
user-invocable: true
metadata:
  graph_root: true
  tier: judgment
  dependencies: []
  capabilities: [compute_subject_identity, judge_acceptance, return_validation_result, persist_verdict]
  effects: [write_verdict_artifact]
  canonical_status: canonical
  disposition: keep
output_contract: 'PASS | FAIL | NOT_PROVEN with criteria, evidence, checked/not_checked, identity, and freshness; optional schemas/verdict.v2.schema.json persistence'
---

# Validate

Independently judge one exact subject against the acceptance in its existing
bead or caller source, return one semantic result, and stop. Validate is the
sole `verdict.v2` writer when persistence is requested. Before the verdict,
read `boundaries.md` in the rpi skill's `references` directory for the state
Validate leaves to the caller.

## Prompt

```text
Validate bead ag-1234 in this fresh context. Intent: the bead text and digest.
Subject: manifest.json from `python3 skills/validate/scripts/validate.py
manifest --root . --include cli/internal/gates`. Author context ctx-a1. Re-run `cd cli && go test
./internal/gates/...`. Return PASS, FAIL, or NOT_PROVEN with evidence; stop.
```

## Preconditions

- The subject is a nonempty implementation candidate: the manifest lists at
  least one entry. Plans, audits, and reviews are subjects only when the
  caller explicitly requested document review.
- The intent source is a caller-owned artifact or a runtime-owned
  content-addressed snapshot; its acceptance digest is derived automatically.
- Author and validator context IDs are explicit, and freshness is attested
  with `source: runtime | caller` and an attester identity. Missing,
  colliding, or unattested identities produce `NOT_PROVEN`: a declared trust
  fact, not cryptographic proof of isolation.

## Cross-family fresh validator (default on risky surfaces)

A second fresh validator from a different model family runs by default when
the diff touches `cli/internal/gates/**`, `scripts/check-*.sh`, `tests/**`,
`skills/*/scripts/**`, `skills/cc-hooks/policies/**`, `lib/**`,
`.github/workflows/**`, or `scripts/security-gate.sh`; elsewhere it is
caller-elected. The runtime floor holds: never `claude -p` or
`claude --print`, directly or indirectly. Adapters and `model_identity`
recording: [references/mechanics.md](references/mechanics.md). With no
authorized live adapter, disclose `diversity_unsatisfied`: off a risky surface
it rides along with a same-model result; on a risky surface a single-family
PASS is `NOT_PROVEN`, and same-family agreement is not convergence. A
single-family FAIL stands.

When the two legs disagree, each reports its own verdict and neither resolves
the split. A risky surface converges only when both legs return PASS, so a
split is never PASS. The split is worked down by repair, and what survives
goes to one council leg that rules on the findings, not the verdicts; the
convergence law then reads the surviving finding set. The plan's
`binding_judge` records the caller's disposition for that outcome and changes
no verdict here. Validate never treats agreement with itself, the absence of a
second verdict, or an elected leg as a tie-break.

## Mutating-check quarantine

Classify every acceptance-listed command as read-only or subject-mutating
before running it: regen scripts, sync scripts, formatters, and anything with
`--force` are mutating until proven otherwise. Run a mutating check only
against a disposable copy or a committed subject, never the judged working
tree (the boundaries appendix records the regen that overwrote a subject).

## Scope disclosure

`not_checked` has exactly one meaning: **in-scope acceptance surface this
validation did not verify**. PASS asserts the whole declared acceptance
surface was verified, so a PASS carries no `not_checked` entries; every other
scope limit has a home that survives inside a PASS: a bounded proof in
`criteria[].reason`, a declared non-goal in the intent source, residual risk
in the report (table in the mechanics reference). Emptying `not_checked` to
obtain PASS is a contract violation: unverified acceptance makes the honest
result `NOT_PROVEN`, and an entry that was never acceptance moves to its home
and stays visible.

## Workflow

1. Derive `subject-manifest.v1` with the helper's `manifest` command (flags
   in the mechanics reference) at the start and again at the end; any
   mismatch is subject mutation and returns `NOT_PROVEN`.
2. Confirm the intent-source digest is unchanged since implementation, every
   cited evidence digest matches the artifact it names, and complete
   changed-path coverage can be derived; otherwise `NOT_PROVEN`.
3. Adjudicate the actual diff: runtime-derived changed paths against the
   intent's scope classes. A proven out-of-scope path is `FAIL`; incomplete
   scope evidence is `NOT_PROVEN`.
4. Inspect the exact subject and evidence. Reported exit codes are claims:
   re-execute the proofs that bear on acceptance. A changed test, gate,
   fixture, golden, tolerance, suppression, or acceptance source must be
   required by the original intent, with green coming from implemented
   behavior; green obtained by weakening acceptance is `FAIL`. Judge every
   acceptance criterion against its own evidence reference; a criterion with
   no evidence of its own is unverified, not passed.
5. Choose exactly one semantic result: `PASS`, `FAIL`, or `NOT_PROVEN`. Return
   it with criterion-level results, findings, evidence references, `checked`,
   `not_checked`, both identities, both context IDs, and the freshness
   attestation. A finding may carry a `class`: one stable short name for the
   defect kind, at most one per finding, reused verbatim when the same kind
   recurs so the traversal can see a class reopen. Omit it or name it; a
   `class` that is present and blank is a finding against this validator, and
   so is a class that does not describe its finding. PASS
   requires distinct identities, explicit freshness, nonempty checked scope,
   nonempty top-level evidence, evidence for every criterion, and an empty
   `not_checked`. A documentation sentence claiming something is published,
   pinned, or proven is an acceptance criterion like any other: it needs a
   check this validator can run, or it is `not_checked`.
6. Only when the caller requests machine-readable evidence or a declared
   downstream consumer requires it, persist canonical `verdict.v2` with the
   helper's `store-verdict` (mechanics reference), then return the artifact
   path and digest with the result. Stop.

Fresh validation is independent judgment over the exact subject, not a replay
of every author command: rerun the risk-critical, uncertain, or thinly
evidenced checks; a digest-bound deterministic receipt may prove routine
facts; replay an expensive full suite only when acceptance requires it. The
repository's full literal CI command set, as quoted in `AGENTS.md`, runs
once, on the final integrated subject.

## It's working if

Observable in the trace, without reading the prose, and the rubric a fresh
independent judge scores this skill against:

- A criterion whose evidence is a justification rather than a proof is named,
  and the result is `NOT_PROVEN` rather than `PASS`.
- Green obtained by widening a tolerance, skipping a case, or re-baselining a
  budget is reported as `FAIL`, never as completion.
- Every scope limit is placed in one of the Scope-disclosure homes; none was
  deleted to reach `PASS`.
- The subject manifest is derived twice, at the start and at the end, and the
  two are compared.

## Boundary

Validate emits no next action, repair, retry, replan, or delivery state (full
list in the boundaries reference); ledger availability cannot change a
verdict's validity.
