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

Run one experiment from the caller's existing intent source through three
responsibilities and stop:

```text
anti-ceremony guard -> Plan -> Implement -> fresh Validate -> bounded repair -> report
```

On `CONTINUE`, the core path remains Plan -> Implement -> fresh Validate ->
bounded repair -> report. RPI invokes the guard exactly once before Plan. It
preserves the original intent and dispatches Plan and Implement at most once.
Validate repeats only inside the repair phase below, under the convergence law
and the caller's `repair_rounds` bound. RPI does not own retries, budgets,
queues, claims, leases, Git, delivery, release, closure, or the caller's next
decision; `repair_rounds` is the caller's declaration, not RPI's budget.

The pure [`scripts/run_once.py`](scripts/run_once.py) reference behavior makes
the dispatch, repair, and stop semantics executable without Git, `ao`, or a
tracker: `invoke_once` for the one bounded experiment, `run_repair_phase` for
the law.

## Admission and phase lock

RPI activates for any request shaped as plan-execute-verify work —
orchestration, worker delegation, "execute this plan", or an explicit
Plan -> Implement -> Validate ask — whenever the goal includes changing the
subject. The caller does not have to name RPI. Research-, audit-, and
review-only delegation is not RPI admission: it produces evidence for a
caller, has no implementation candidate, and never earns a verdict.

Once the caller has accepted a plan — including a duel or design synthesis —
Plan is closed for that intent. Every subsequent lane must return
implementation evidence: diffs, commits, test results, or factual receipts.
Dispatching another planning, audit, or review lane over the same intent
requires new explicit caller authorization; a review comment is never that
authorization by itself.

## Contract

1. Invoke anti-ceremony's artifact-free quick guard once with the caller
   outcome, proposed process work, remaining proof, and stop condition. On
   `STOP`, dispatch no core phase, report `NOT_PLANNED` with the guard's
   one-sentence reason, and stop. On `CONTINUE`, proceed without adding an
   artifact, retry, repair, delivery, tracker, or Git action.
2. Resolve the existing bead or caller intent. Invoke Plan once only if that
   source needs shaping; Plan updates the same source or proposes an amendment.
   It creates no AgentOps packet. Preserve a durable caller-owned source by
   reference and digest; only when no durable source exists does the runtime
   snapshot the exact resolved source bytes under their digest before
   dispatching Implement or a fresh Validate context. If usable intent cannot
   be established, report `NOT_PLANNED` and stop.
3. Invoke Implement once with the resolved intent. It performs one bounded
   experiment; the runtime derives subject identity and check receipts. If no
   subject is built, report `NOT_BUILT` and stop.
4. Invoke Validate once in a context distinct from the author's context. Pass
   the intent reference and digest, exact subject manifest, factual receipts,
   validator identity, and freshness attestation.
5. Enter the bounded repair phase. On a converged result, report and stop. On
   `FAIL` or `NOT_PROVEN` with findings, repair the named findings and
   re-validate freshly while the convergence law admits another round; stop
   when converged, stopped by the law, or out of `repair_rounds`. Return the
   current validation result, the open findings, and a short report. Persist
   and link `verdict.v2` only when the caller requests machine-readable
   evidence or a declared downstream consumer requires it.

`NOT_PLANNED` and `NOT_BUILT` are report statuses, never semantic verdicts.
A caller may revise the bead or caller intent and start a new invocation. RPI
never creates a parallel revision artifact or selects the next work itself.

## The convergence law

A repair round is admitted only while all hold:

1. `rounds_used < repair_rounds` (caller-declared, default 2).
2. The open finding set, keyed by stable `findings[].id` (union of the fresh
   and cross-family validators), is not larger than the previous round's.
3. No finding id closed in an earlier round reopens.
4. Between rounds the subject-manifest digest changed (generated-only changes
   count) or, for `NOT_PROVEN`, new digest-bound evidence resolved a named gap.

Converged: the fresh validator returns PASS and, on a risky surface, the
cross-family validator also returns PASS. On any violation of 1-4 RPI stops and
reports the current status. `checked` carries one line per round
(`repair round N: k open findings`); open findings ride in the validation
result and the report; `not_checked` keeps its meaning. A reworded finding with
the same id is the same finding. The orchestrating context fixes; judge legs
never mutate the subject. No third judge, no escalation, no auto-replan.

## Cross-family validation

Risky surfaces default to a cross-family fresh validator: `cli/internal/gates/**`,
`scripts/check-*.sh`, `tests/**`, `skills/*/scripts/**`,
`skills/cc-hooks/policies/**`, `lib/**`, anything `security-gate.sh` scans.
[`validate`](../validate/SKILL.md) owns the dispatch table. No authorized live
adapter means `diversity_unsatisfied`, which on a risky surface is `NOT_PROVEN`.

## Waves

RPI executes one traversal. A multi-wave intent runs one wave per `crank`
invocation: the caller selects the wave and the `repair_rounds` bound, crank
forwards both, invokes RPI per lane, returns wave evidence, and stops. RPI never
selects a wave or queues the next one, and never extends the caller's bound.

## Anti-ceremony boundary

The hard [`anti-ceremony`](../anti-ceremony/SKILL.md) dependency owns the quick
guard and its explicit-only full honesty audit. RPI does not duplicate that
judgment or turn each component, gate failure, or specialist comment into a new
planning artifact. A terminal caller goal may remain one bounded experiment
across several source owners when they serve one outcome and one acceptance
boundary.

If control artifacts or fresh-validation cycles are multiplying faster than
implementation evidence, stop dispatching more lanes. Return to one
outcome-level intent and continue with targeted deterministic checks, reserving
the full integration check and fresh validation for the frozen subject. This
changes orchestration cost, never acceptance, exact identity, fail-closed
scope, or validation authority.

## Spiral breaker

The spiral breaker fires on a convergence-law violation, or when two
consecutive rounds produce no change to the subject digest and no new
digest-bound evidence. It never fires on a verdict count: a `FAIL` or a
`NOT_PROVEN` that is being repaired under the law is progress, not a spiral,
and repeated control artifacts (plans, audits, reviews, prompts, reports) with
no new implementation evidence are the spiral. Terminate the run and report
`NOT_BUILT` when no implementation subject exists; when a subject exists, stop
and report its current status without dispatching another lane. RPI owns no
lane budget and no retry policy, and never extends the caller's
`repair_rounds`.

## Delegation boundaries

Delegate with minimal context: a lane receives the frozen intent reference and
the established facts it needs, never the orchestrator's full conversation
history. If a lane cannot proceed from the intent alone, report that the plan
failed the fresh-context test and stop; do not pad it with chat transcript or
start another planning lane without explicit caller authorization.

Lanes whose write scopes share a regen surface (the same generated outputs,
mirrors, or manifests) serialize; only lanes with disjoint source scopes and
disjoint regen surfaces may run in parallel.

## Invariants

- Acceptance and its runtime-derived digest do not change between phases or
  between repair rounds; a repair moves the subject, never the acceptance.
- The anti-ceremony guard runs once before Plan; `STOP` dispatches none of Plan,
  Implement, or Validate, while `CONTINUE` preserves their order.
- The runtime derives complete changed-path coverage or Validate returns
  `NOT_PROVEN`.
- A proven change outside `write_scope` makes the verdict `FAIL`.
- PASS requires nonempty distinct author and validator context IDs plus an
  explicit freshness attestation.
- Optional Premortem, Postmortem, Council, genie, factory, tracker, and runtime
  adapters are caller-selected. They do not alter phase order or core outcomes.
  When a factory adapter is selected, work enters it through that factory's
  coordinator (for Gas City, the Mayor — see
  [using-gc](../using-gc/SKILL.md)); RPI hands over intent and never dispatches
  factory runs itself.
- Learn is an optional later consumer of verdict collections and is not part of
  this invocation.

## Report

RPI has one required report surface and one optional representation:

1. **Interactive response:** return the result to the caller in natural
   language. This is the default assistant response.
2. **Machine artifact:** return or persist the exact `rpi-report.v1` object
   only when the caller requests machine-readable evidence or a declared
   adapter consumes it. The schema ships in a repo checkout at
   `schemas/rpi-report.v1.schema.json`; the minimal required shape is:

   ```json
   {
     "schema_version": "rpi-report.v1",
     "status": "PASS",
     "intent_ref": "<durable-source-ref-or-fallback-snapshot-ref>",
     "acceptance_digest": "<64-hex-char-sha256-or-null>",
     "subject_manifest_digest": "<64-hex-char-sha256-or-null>",
     "verdict_ref": "<verdict-location-or-null>",
     "verdict_digest": "<64-hex-char-sha256-or-null>",
     "checked": ["<criterion satisfied by evidence>"],
     "not_checked": ["<criterion not covered>"]
   }
   ```

   `intent_ref` remains required: it names the durable caller-owned source when
   one exists, otherwise the content-addressed fallback snapshot. `status` is
   one of `PASS | FAIL | NOT_PROVEN | NOT_PLANNED | NOT_BUILT`; the three digest
   fields, when present, are 64-character lowercase hex SHA-256 strings;
   `checked` and `not_checked` are arrays of strings. All nine keys are required
   (use `null` for an inapplicable ref or digest), and no
   additional properties are allowed.

Lead the interactive response with the status and one sentence stating the
caller-visible outcome. Lead with the subject, not the process: production
paths changed, commits, test results, and acceptance criteria satisfied or
remaining. A rising artifact count over an unchanged subject is a stop
signal, not progress. Follow with only the strongest proof, any material
unchecked scope, and a clickable verdict reference when one exists. Name why
no subject exists for `NOT_PLANNED` or `NOT_BUILT`; for a guard `STOP`, use its
one-sentence reason. Keep the response to one short paragraph or at most four
bullets.

When no machine artifact was requested, do not create a hidden one. Raw digests,
schema fields, and exhaustive check lists stay out of the interactive response
unless an integrity failure makes one necessary to explain the result.

Do not append a next action. The caller owns continuation.
