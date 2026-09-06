---
name: rpi
description: 'Coordinate one RPI traversal: one bounded Plan and Implement experiment, then fresh Validate and a bounded repair phase to convergence. Triggers: "run rpi", "run one traversal", "execute this plan", orchestration or worker delegation that implements changes.'
practices:
- bdd-gherkin
- tdd
- design-by-contract
hexagonal_role: domain
consumes:
- anti-ceremony
- plan
- implement
- validate
produces:
- rpi-report.v1
context_rel:
- kind: customer-of
  with: anti-ceremony
- kind: customer-of
  with: plan
- kind: customer-of
  with: implement
- kind: customer-of
  with: validate
skill_api_version: 1
user-invocable: true
metadata:
  graph_root: true
  tier: meta
  dependencies: [anti-ceremony, plan, implement, validate]
  capabilities: [orchestrate_once, report]
  effects: [invoke_anti_ceremony_guard, dispatch_core_phases]
  canonical_status: canonical
  disposition: keep
output_contract: 'concise human-readable result; optional rpi-report.v1 when a caller or declared consumer requests machine-readable evidence'
---

# RPI

Run one experiment from the caller's existing intent source and stop:

```text
anti-ceremony guard -> Plan -> Implement -> fresh Validate -> bounded repair -> report
```

RPI invokes the guard exactly once before Plan, preserves the original intent,
and dispatches Plan and Implement at most once; Validate repeats only inside
the repair phase, under the convergence law and the caller's `repair_rounds`.
Read [references/boundaries.md](references/boundaries.md), the ownership and
delegation boundary shared by the core skills, before dispatch.
[`scripts/run_once.py`](scripts/run_once.py) makes dispatch, repair, and the
four mechanical stop rules executable without Git, `ao`, or a tracker; the
class rule is the orchestrator's judgment, not the script's.

## Prompt

```text
Run rpi on bead ag-1234 ("ao gate check lists the probe-coverage row").
Intent: the bead. Scope: cli/internal/gates/** plus docs/CI-CD.md. First check:
cd cli && go test ./internal/gates/... Fresh validator in a distinct context,
plus a cross-family leg (the scope is a risky surface). repair_rounds=2.
```

## It's working if

- The transcript shows one `anti-ceremony` call, then at most one `plan` and
  one `implement` dispatch.
- The validator's context ID differs from the author's, and the report opens
  with `status:` and changed paths, not a digest.
- Each round appends one `repair round N: k open findings` line to `checked`,
  `k` never grows, and the run ends on `converged`, a law violation, or
  `repair_rounds`, with no next action after the evidence.

## Admission and phase lock

RPI activates for any plan-execute-verify request that changes the subject
(orchestration, worker delegation, "execute this plan"), named or not.
Research-, audit-, and review-only delegation produces evidence for a caller
and earns no verdict.

Once the caller accepts a plan (a duel or design synthesis included),
Plan is closed for that intent: every later lane returns implementation
evidence (diffs, commits, test results, receipts). Another planning, audit, or review
lane over the same intent needs new explicit caller authorization; a review
comment alone is not that.

## Contract

1. Invoke anti-ceremony's artifact-free quick guard once with the caller
   outcome, proposed process work, remaining proof, and stop condition. On
   `STOP`, dispatch no core phase, report `NOT_PLANNED` with the guard's
   one-sentence reason, and stop. On `CONTINUE`, proceed and add nothing else.
2. Resolve the existing bead or caller intent. Invoke Plan once only if the
   source needs shaping; Plan updates that source or proposes an amendment and
   creates no AgentOps packet. Without usable intent, report `NOT_PLANNED`.
   Before Implement or a fresh Validate, always bind the intent: a durable
   caller-owned source by reference and digest, or, only when no durable
   source exists, the exact resolved bytes snapshotted by the runtime under
   their digest.
3. When the write scope touches a risky surface (the short list
   [`validate`](../validate/SKILL.md) names), have one fresh judge read the
   frozen plan before Implement. A blocking finding sends the plan back to the
   caller as `NOT_PLANNED`, naming that finding. The caller may waive the read,
   and the report says so. Every terminal report says whether that read was not
   required, clean, blocking, waived, or never finished, so a waived or dead
   read is never taken for a clean one.
4. Invoke Implement once: one bounded experiment; the runtime derives subject
   identity and check receipts. After Implement, and again after each repair
   round, run `bash scripts/evidence-orphans.sh <changed paths>` and put its
   output in the check receipts, so the validator and the caller both see what
   evidence this change orphaned. With no subject built, report `NOT_BUILT`.
5. Invoke Validate once in a context distinct from the author's, passing the
   intent reference and digest, exact subject manifest, receipts, validator
   identity, and freshness attestation.
6. Enter the bounded repair phase: on `FAIL` or `NOT_PROVEN` with findings,
   repair the named findings and re-validate freshly while the law admits
   another round; stop when converged, stopped by the law, or out of
   `repair_rounds`. Persist `verdict.v2` only when the caller requests
   machine-readable evidence or a declared consumer requires it.

`NOT_PLANNED` and `NOT_BUILT` are report statuses, never semantic verdicts.
A caller may revise the intent and start a new invocation.

## The convergence law

A repair round is admitted only while all hold:

1. `rounds_used < repair_rounds` (caller-declared, default 2).
2. The open finding set, keyed by stable `findings[].id` (union of the fresh
   and cross-family validators), is not larger than the previous round's.
3. No finding id closed in an earlier round reopens.
4. Between rounds the subject-manifest digest changed (generated-only changes
   count) or, for `NOT_PROVEN`, new digest-bound evidence resolved a named gap.
5. No class of finding closed in an earlier round comes back on a new finding.

Validators name a class for each finding: one short stable name for the kind of
defect, reused word for word when the same kind recurs, so renaming a defect
every round cannot hide it. Rule 5 is about the kind, not the wording. A class
that was closed coming back on a fresh finding stops repair even when the open
set never grew and no earlier finding reopened. Stop repairing and go back to
Plan: the design is wrong, not the patch.

Converged: the fresh validator returns PASS and, on a risky surface, so does
the cross-family validator. On any violation of 1-5 RPI stops and reports the
current status. `checked` carries one line per round
(`repair round N: k open findings`); open findings ride in the result and the
report. A reworded finding with the same id is the same finding. Acceptance
and its digest stay fixed: a repair moves the subject. The orchestrating
context fixes; judge legs only read. RPI convenes no further judge of its own,
does not escalate, and does not auto-replan.

## Cross-family validation

Risky surfaces default to a cross-family fresh validator: `cli/internal/gates/**`,
`scripts/check-*.sh`, `tests/**`, `skills/*/scripts/**`,
`skills/cc-hooks/policies/**`, `lib/**`, `.github/workflows/**`, `scripts/security-gate.sh`.
[`validate`](../validate/SKILL.md) owns the surface list. No authorized live
adapter means `diversity_unsatisfied`, which on a risky surface is `NOT_PROVEN`.

Two judges disagreeing is the orchestrator's decision, and it is made in the
open. Both reads go in the report, each with its own verdict, alongside what
was decided and why. A risky surface still converges only when both judges
pass, so a split is never a PASS and no finding leaves the open set because one
judge was preferred.

## Judgment dispatch

| Condition | Leg |
|---|---|
| the write scope reaches a risky surface at Plan exit, a broad or unbounded scope included | [`premortem`](../premortem/SKILL.md) before Implement; a blocking finding is `NOT_PLANNED` |
| the two judges split | the orchestrator decides in the open and records both reads; [`council`](../council/SKILL.md) is available when the caller selects it |
| an irreversible landing decision | `one-way-door`, caller-selected, outside the traversal |

## Waves

RPI executes one traversal. A multi-wave intent runs one wave per `crank`
invocation: the caller selects the wave and the `repair_rounds` bound, crank
forwards both, invokes RPI per lane, returns wave evidence, and stops.
The caller selects each wave; RPI never extends the caller's bound.

## Spiral breaker

The hard [`anti-ceremony`](../anti-ceremony/SKILL.md) dependency owns the quick
guard; RPI reuses that judgment instead of turning each component, gate
failure, or specialist comment into a new planning artifact, and one terminal
goal may span several source owners as one bounded experiment.

The spiral breaker fires on a convergence-law violation, or when two
consecutive rounds change neither the subject digest nor the digest-bound
evidence, never on a verdict count: a `FAIL` or `NOT_PROVEN` under repair is
progress; repeated control artifacts with no new implementation evidence are
the spiral. Report `NOT_BUILT` when no subject exists; otherwise report the
subject's current status without dispatching another lane, keeping the full
integration check and fresh validation for the frozen subject.

## Report

1. **Interactive response:** return the result to the caller in natural
   language. This is the default assistant response.
2. **Machine artifact:** return or persist the exact `rpi-report.v1` object
   only when the caller requests machine-readable evidence or a declared
   adapter consumes it; `schemas/rpi-report.v1.schema.json` (repo checkout)
   owns its nine-key shape and `status` set.

Say in the report what this change orphaned: the evidence the plan budgeted to
recapture, and the evidence the orphan receipt actually named after Implement
and after each repair round.

Lead with the status and one sentence naming the caller-visible outcome, then
the subject: paths changed, commits, test results, acceptance satisfied or
remaining. A rising artifact count over an unchanged subject is a stop
signal, not progress. Add only the strongest proof, material unchecked scope,
and a clickable verdict reference when one exists; for `NOT_PLANNED`,
`NOT_BUILT`, or a guard `STOP`, say why no subject exists in one sentence.
One short paragraph or at most four bullets, ending with the evidence.
When no machine artifact was requested, do not create a hidden one.
