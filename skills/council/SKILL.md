---
name: council
description: 'Collect independent perspectives for an explicitly high-stakes or contested judgment. Triggers: "council", "multi-judge review", "independent perspectives".'
practices: [llm-eval-harness, design-by-contract]
hexagonal_role: domain
consumes: [explicit-question, evidence]
produces: [council-report.v1]
context_rel: []
skill_api_version: 1
user-invocable: true
metadata:
  graph_root: true
  tier: judgment
  dependencies: []
  capabilities: [collect_independent_judgments, synthesize_disagreement]
  effects: [read_declared_evidence_packet, dispatch_authorized_bounded_model_judges, write_advisory_council_report]
  canonical_status: canonical
  disposition: keep_strategy
output_contract: council-report.v1 JSON validated by skills/council/scripts/validate-output.sh
---

# Council

Council is an optional judgment strategy, not a lifecycle or delivery gate. Use
it when one fresh validator is insufficient for a named irreversible,
high-blast-radius, or genuinely contested decision. Do not convene a council for
a routine or reversible decision that a single fresh validator can settle: the
cost of independent contexts is warranted only by a named one-way door.

## Constraints

- **Why judges need identical bounded evidence.** Freeze one allowlisted packet of at most 20 sources and 256 KiB total. Record
  its source identities, exact byte count, and SHA-256. Reject oversized,
  unreadable, secret-bearing, or out-of-allowlist input before any dispatch.
- **Why identity must be declared.** The caller authorizes 2–5 judge attempts and declares the adapter and model
  identity for each. Record one authorization ID; unavailable adapters/models
  become explicit `error` attempts and are never silently substituted.
- Each judge has a 300-second maximum timeout and 32 KiB output ceiling; the
  round has a 900-second maximum deadline. A local judge runs in a new process
  group. Timeout, cancellation, or overflow sends TERM then KILL to the whole
  group and waits for cleanup; remote adapters must return equivalent confirmed
  cancellation. No third-party command is inferred from evidence text.
- Judges are read-only. Dispatch receives only the frozen packet and may write
  only its bounded scratch response. Any subject mutation, cleanup failure,
  un-reaped process, packet drift, or undeclared network/credential/data access
  stops the round as `insufficient` with no consensus claim.

Local no-network judges use Validate's
[`run-check` bounded runner](../validate/scripts/validate.py). Remote adapters
must enforce the same exact-command, output, deadline, cancellation, and
cleanup contract plus their caller-approved endpoint and credential allowlists.

1. Freeze one question, acceptance surface, evidence set, subject digest, and
   the bounded packet/dispatch declaration above.
2. Give each declared judge an independent context and the same bounded packet.
3. Require each judge to cite evidence, disclose omissions, and return its own
   judgment without seeing other answers first.
4. Synthesize agreement and disagreement without majority laundering. Preserve
   minority evidence and unresolved assumptions.
5. Write `council-report.v1` and return it to the caller.

## Methodology-weighted agreement

Agreement across differing evidence methodologies counts more than agreement
within one. Record each judge's evidence methodology (for example: static
reading, executing the subject, tracing history) alongside its judgment. A
consensus claim must name at least two distinct methodologies among its
supporting judges; otherwise report it as single-method agreement and weight
it as one confirmation, however many judges share it. The named failure mode
is echo consensus: unanimous judgment produced from identical inputs by one
shared method, laundered as independent confirmation.

## Model-diversity axis

When the caller pins judges to model profiles, record each judge's
`model_identity` beside its methodology and context ID (see
the `agent-native` model-dispatch recipe).
Cross-model agreement is an additional diversity axis: single-model unanimity
is weighted as one confirmation with the same anti-echo-consensus rationale,
regardless of how many judges share that model. If a requested profile has no
live adapter, disclose `diversity_unsatisfied` on the report and continue
single-model — never silently, never via `claude -p`.

## Fresh sessions per round

Every judging round uses fresh judge contexts with new context IDs, distinct
from the author, the synthesizer, and every prior round. A judge that has
seen another judge's answer, or its own prior-round answer, is no longer
independent: exclude its judgment from agreement counting and admit it only
as labeled commentary. Reused or colliding context IDs are a checkable stop
condition — repair the isolation or report the round as non-independent.

## Synthesis section

The report ends with an explicit consensus/divergence synthesis: consensus
points with their methodology spread, divergence points with each side's
cited evidence, minority findings preserved in their own words, and
unresolved assumptions. Synthesis is complete when every judge finding lands
in exactly one of those buckets; a finding silently dropped from synthesis is
majority laundering.

## Output

- **Artifact directory:** `.agents/scratch/council/<run-id>/`.
- **Filename:** `council-report.json`.
- **Format:** `council-report.v1` JSON — the frozen question and subject digest,
  every judge's context ID, evidence methodology, cited evidence, and disclosed
  omissions, plus the consensus/divergence/minority/unresolved synthesis. It
  carries no `verdict`, `readiness`, or `PASS` field; the validator rejects one.
- **Validation command:**
  `skills/council/scripts/validate-output.sh <council-report.json>`.

A judge that times out, errors, or returns an evidence-free judgment is excluded
from agreement counting and recorded as non-returning; if fewer than two
independent judgments remain, set `round_status` to `insufficient`, keep the
synthesis arrays empty, disclose the missing attempts, and stop rather than
synthesize a thin consensus.

## Quality checks

- Packet byte/source bounds, authorization, declared adapter/model identities,
  judge/round deadlines, and output caps are present and within schema limits.
- Every attempt is represented exactly once with a unique context ID, factual
  status, output byte count, and confirmed process/adapter cleanup.
- `sufficient` has at least two completed independent judgments;
  `insufficient` carries no consensus, and neither status mutates the subject.

## Boundary

Council does not mint a verdict of any version — no `PASS`/`FAIL`/`NOT_PROVEN`,
no `verdict.v*` — edit the subject, retry work, choose a next action, or
authorize Git, closure, release, or delivery. When Council is used as a Validate
strategy, one accountable fresh validator consumes its report and Validate
remains the sole semantic result owner and the only optional `verdict.v2`
writer.
