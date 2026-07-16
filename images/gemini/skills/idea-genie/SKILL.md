---
name: idea-genie
description: 'Generate an evidence-grounded opportunity portfolio for an open-ended product or engineering question, or challenge a consequential idea with sealed independent perspectives, cross-review, and preserved dissent. Triggers: "idea genie", "what should we build", "supported opportunities", "challenge this idea", "compare independent proposals", "stress-test a one-way door".'
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
  effects: [write_idea_portfolio]
  canonical_status: canonical
  disposition: keep_strategy
output_contract: idea-portfolio.v1 JSON validated by skills/idea-genie/scripts/validate-output.sh (elicit mode), or idea-challenge.v1 JSON validated by skills/idea-genie/scripts/validate-challenge.sh (duel mode)
---

# Idea Genie

One canonical root for idea work: elicit an evidence-grounded opportunity
portfolio, or challenge a consequential idea with sealed independent
perspectives. Both modes explore and advise; neither selects, schedules,
tracks, implements, or validates work.

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

1. State the question, constraints, non-goals, and sources.
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
   identifiers. Each produces its perspective before any is revealed.
3. Reveal the sealed perspectives and cross-review by evidence, reversibility,
   system fit, failure modes, and cost.
4. Attempt concrete refutations. Preserve disagreements, failed refutations,
   and minority reasoning.
5. Write `idea-challenge.v1`, validate it, and pass the artifact to Plan as one
   optional input alongside research and operator intent.

For a cheap two-way door, emit the lightweight packet directly after one fresh
challenge. Do not manufacture panel ceremony.

### Output Specification

- **Artifact directory:** `.agents/ideas/<run-id>/`
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

### Do not

- Let perspectives see one another before sealed generation completes.
- Convert consensus, transport availability, or a self-score into readiness.
- Require orchestration infrastructure for a reversible choice.

## References

- [Idea Genie behavior](references/idea-genie.feature)
- [Idea challenge behavior](references/idea-challenge.feature)
