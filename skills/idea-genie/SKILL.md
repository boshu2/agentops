---
name: idea-genie
description: 'Generate evidenced opportunities or challenge an idea with sealed perspectives. Triggers: "idea genie", "what should we build", "challenge this idea", "compare proposals".'
practices: [lean-startup, bdd-gherkin, design-by-contract, llm-eval-harness, adr]
hexagonal_role: domain
consumes: [repo-context, task-question, idea-portfolio.v1]
produces: [idea-portfolio.v1, idea-challenge.v1]
context_rel:
- kind: customer-of
  with: research
- kind: supplier-to
  with: plan
skill_api_version: 1
user-invocable: true
metadata:
  tier: execution
  dependencies: []
  capabilities: [generate_evidenced_options, dueling_idea_genies]
  effects: [read_declared_evidence_packet, optionally_query_approved_sources, dispatch_authorized_bounded_model_challenges, write_idea_portfolio, write_idea_challenge]
  canonical_status: canonical
  disposition: keep_strategy
output_contract: idea-portfolio.v1 JSON validated by skills/idea-genie/scripts/validate-output.sh (elicit mode), or idea-challenge.v1 JSON validated by skills/idea-genie/scripts/validate-challenge.sh (duel mode)
---

# Idea Genie

One canonical root for idea work: elicit an evidence-grounded opportunity
portfolio, or challenge a consequential idea with sealed independent
perspectives. Both modes explore and advise; neither selects, schedules,
tracks, implements, or validates work.

## Constraints

- **Why comparable outputs need a frozen packet.** Freeze an allowlisted evidence packet before either mode: at most 20 sources
  and 256 KiB, with exact byte count and SHA-256. Reject unreadable,
  secret-bearing, oversized, or out-of-allowlist content before generation.
- **Why model substitution is misleading.** Record the caller authorization ID and declared limits. Elicit runs at most
  three novelty passes, retains at most ten candidates, and emits at most
  64 KiB. A two-way challenge uses one fresh context; a one-way challenge uses
  2–4 declared judge attempts. Every adapter and model identity is named before
  dispatch; unavailable entries become explicit errors, never substitutions.
- Each challenge attempt has a 300-second maximum deadline and 32 KiB output
  cap; the whole challenge has a 900-second maximum. Local attempts run in new
  process groups. Timeout, cancellation, or overflow terminates/reaps the group;
  remote adapters must confirm equivalent cancellation.
- Generation is read-only over the subject and may write only the declared
  bounded scratch artifact. Packet drift, subject mutation, cleanup failure,
  un-reaped judges, or undeclared network/credential/data access produces an
  explicit `insufficient` result with no synthesis or Plan-ready route.

Local no-network challenges use Validate's
[`run-check` bounded runner](../validate/scripts/validate.py). Remote adapters
must enforce equivalent output, deadline, cancellation, and cleanup bounds plus
their caller-approved endpoint and credential allowlists.

## Modes

| Trigger phrases | Mode | Output contract |
|---|---|---|
| "idea genie", "what should we build", "supported opportunities" | elicit (single genie) | `idea-portfolio.v1` via `scripts/validate-output.sh` |
| "challenge this idea", "compare independent proposals", "stress-test a one-way door" | duel (adversarial challenge) | `idea-challenge.v1` via `scripts/validate-challenge.sh` |

Elicitation is the entry mode. Dueling is an optional escalation for a
consequential choice, typically consuming an `idea-portfolio.v1` or a framed
question.

## Elicit mode

Generate a small portfolio of evidenced options.

1. State the question, constraints, non-goals, and sources. Hydrate only the sources this question needs and cite them; no merged context store.
2. Separate cited observations from assumptions.
3. Give each candidate its supporting evidence, overlap with existing
   capabilities, and one normal or edge scenario.
4. Run a novelty pass, merge equivalents, and discard unsupported ideas.
5. Stop when no materially new evidenced candidate appears.
6. Write and validate `idea-portfolio.v1`, then return it to the caller or Plan.

An empty `no-new-work` portfolio is valid. Plan alone may incorporate a selected
option into the existing bead or caller intent.

## Duel mode

Produce independent challenges for a consequential choice. The result is
advisory evidence for Plan. It never decides whether a plan is ready and never
turns a later optional Premortem challenge into an approval gate.

### Constraints

- Keep generation sealed until every perspective is complete to prevent later
  proposals from anchoring on earlier ones.
- Preserve dissent and concrete refutation attempts because Plan must see alternatives
  that synthesis might otherwise erase.
- Keep reversible choices lightweight because they do not warrant a pane manager,
  messaging service, council, or model-family rule.
- Emit no readiness, approval, quorum, retry, budget, helper, delivery, or
  tracker state because this strategy supplies evidence rather than lifecycle
  authority.

### Workflow

1. Freeze the question, constraints, evidence paths, and comparison rubric.
2. For a one-way door, create at least two fresh contexts with distinct context
   identifiers. Each produces its perspective before any is revealed. When the
   caller pins perspectives to model profiles, record each perspective's
   `model_identity` (see the `agent-native` model-dispatch recipe); a
   duel may use two distinct models on request. Sealed generation is unchanged:
   no perspective may see another before reveal. Unavailable profiles → disclose
   and continue single-model.
3. Reveal the sealed perspectives and cross-review by evidence, reversibility,
   system fit, failure modes, and cost.
4. Attempt concrete refutations. Preserve disagreements, failed refutations,
   and minority reasoning.
5. Write `idea-challenge.v1`, validate it, and pass the artifact to Plan as one
   optional input alongside research and operator intent.

For a cheap two-way door, emit the lightweight packet directly after one fresh
challenge. Do not manufacture panel ceremony.

### Output Specification

- **Artifact directory:** `.agents/scratch/ideas/<run-id>/`
- **Filename:** `idea-challenge.json`
- **Format:** `idea-challenge.v1` JSON with route-specific fields enforced by
  the validator
- **Validation command:**
  `skills/idea-genie/scripts/validate-challenge.sh <idea-challenge.json>`
- **Downstream handoff:** `handoff.owner` is exactly `plan`; Plan may accept,
  reject, or combine the advisory evidence

### Quality

- One-way packets prove distinct context IDs and cross-review another
  perspective by named dimensions.
- Dissent and refutation attempts remain explicit.
- The packet contains no semantic readiness field or decision.
- The validator passes before handoff to Plan.

## Quality checks

- Packet, pass/candidate, output, judge-count, model/adapter, and deadline bounds
  are present and within the validators' hard maxima.
- Every dispatched context has a unique ID, factual completion/error/timeout
  status, bounded output bytes, and confirmed cleanup.
- An insufficient or unclean challenge contains no cross-review synthesis and
  cannot claim the normal Plan handoff route; neither mode mutates the subject.

### Do not

- Let perspectives see one another before sealed generation completes.
- Convert consensus, transport availability, or a self-score into readiness.
- Require orchestration infrastructure for a reversible choice.

## References

- [Idea Genie behavior](references/idea-genie.feature)
- [Idea challenge behavior](references/idea-challenge.feature)
