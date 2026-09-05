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
[`scripts/run_once.py`](scripts/run_once.py) is the reference for the repair
law: it makes round admission, class reopening, and stop executable without
Git, `ao`, or a tracker. It implements that law only, not the premortem,
council, or receipt legs.

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
3. On a risky write scope, dispatch the Plan-exit premortem leg below before
   Implement. A nonempty `blocking` list returns `NOT_PLANNED` naming those
   findings. A caller may declare `premortem: skip`. Every terminal report
   carries `premortem: not-required | required | clean | blocking | skipped | failed`, so a
   skipped or failed leg is never read as a clean one.
4. Invoke Implement once: one bounded experiment; the runtime derives subject
   identity and check receipts. With no subject built, report `NOT_BUILT`.
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
5. No finding class closed in an earlier round reappears under a new id.

`findings[].class` is optional: a stable short name for the defect kind,
either absent or a real name, never present and blank. A present-but-blank
class makes the round invalid, because a blank class defeats rule 5 in
silence. Rule 5 tests continuous rename: a resolved id's class reappearing on
a new id, while no surviving prior id still carries that class, is
`class_reopened`. One round can carry both a reopened id and a reopened class;
repair stops on either, and the caller returns to Plan, because the design is
wrong, not the patch.

Converged: the fresh validator returns PASS and, on a risky surface, so does
the cross-family validator. On any violation of 1-5 RPI stops and reports the
current status. `checked` carries one line per round
(`repair round N: k open findings`); open findings ride in the result and the
report. A reworded finding with the same id is the same finding. Acceptance
and its digest stay fixed: a repair moves the subject. The orchestrating
context fixes; judge legs only read. No judge beyond the council leg, no
escalation, no auto-replan.

## Cross-family validation

Risky surfaces default to a cross-family fresh validator: `cli/internal/gates/**`,
`scripts/check-*.sh`, `tests/**`, `skills/*/scripts/**`,
`skills/cc-hooks/policies/**`, `lib/**`, `.github/workflows/**`, `scripts/security-gate.sh`.
[`validate`](../validate/SKILL.md) owns the surface list. No authorized live
adapter means `diversity_unsatisfied`, which on a risky surface is `NOT_PROVEN`.

The law stands over both legs. A risky surface converges only when both legs
return PASS; a split never certifies PASS; no finding leaves the open set
because a judge was elected. `binding_judge` is a field of the Plan output,
bound into the plan identity, and a caller argument that disagrees with it
refuses the traversal. It declares the caller's disposition for a split that
survives repair, applies only on a risky scope, and is carried in the report.
It is never a verdict override and never mutates a leg's verdict.

A risky split that survives repair goes to one
[`council`](../council/SKILL.md) leg that adjudicates findings, not verdicts.
The leg receives a bounded packet (acceptance, write scope, runtime-derived
changed paths, criteria, and both legs' findings with evidence references,
marked untrusted) and returns one ruling per finding. The traversal validates
the shape of what comes back (exactly one ruling per finding id, a duplicated
id refuses the whole set, ids checked against the table, evidence references
kept verbatim and never resolved) and records it under `council.rulings`. It
closes nothing. The verdict and the open finding set are exactly what the
repair phase left them, and the rulings are there for the caller's next intent
to read.

The council closed findings on cited evidence for five rounds, and each round
of hardening that path drew a new defect of the same kind. By this traversal's
own convergence law a class that reopens after repair means the design is
wrong, so the closure was cut rather than hardened again. A convergence-law
stop convenes no council at all: the run already failed to converge, and a
third judge on top of that is the escalation the law forbids. Council mints no
verdict.

## Judgment dispatch

| Condition | Leg |
|---|---|
| risky write scope at Plan exit: a `write_scope` glob intersects a risky-surface glob, and a bare `**` or `*` glob counts as an intersection | [`premortem`](../premortem/SKILL.md) before Implement, returning `{blocking: [{id, class, summary}], notes: [...]}`; nonempty `blocking` is `NOT_PLANNED` |
| a risky split that survives repair | `council`, one leg, returning `{id, ruling: real \| not_real \| not_proven, evidence_refs}` per finding |
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
   owns its exact shape and `status` set.

The workflow result is the camelCase object the adapter returns. It carries
`status`; `premortem`; `dissent`, the judge split with each leg's own verdict;
`plannedOrphans`, the evidence the frozen plan said this write scope would
orphan, budgeted as recapture work before any code was written;
`orphanedEvidence`, the bound evidence this change actually invalidated with a
`cause` per entry; and `stopReason` from a fixed enum. Each is empty or null when
nothing applies; none is omitted to keep a result short. Those keys belong to
the workflow result alone: `rpi-report.v1` keeps its nine snake_case keys
unchanged.

Lead with the status and one sentence naming the caller-visible outcome, then
the subject: paths changed, commits, test results, acceptance satisfied or
remaining. A rising artifact count over an unchanged subject is a stop
signal, not progress. Add only the strongest proof, material unchecked scope,
and a clickable verdict reference when one exists; for `NOT_PLANNED`,
`NOT_BUILT`, or a guard `STOP`, say why no subject exists in one sentence.
One short paragraph or at most four bullets, ending with the evidence.
When no machine artifact was requested, do not create a hidden one.
