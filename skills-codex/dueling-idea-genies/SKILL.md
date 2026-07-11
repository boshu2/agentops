---
name: dueling-idea-genies
description: Challenge a contested one-way-door idea with
---
# Dueling Idea Genies

Produce independent challenges for a consequential choice. The output is
evidence for the existing plan-pawl decider; this skill never assigns
PASS/REDO/BLOCKED and never counts the author as an independent perspective.

## Constraints

- Keep generation sealed until every perspective is complete to prevent later
  proposals from anchoring on earlier ones.
- Preserve dissent and concrete refutation attempts because the plan-pawl, not
  this skill, owns the decision.
- Keep two-way doors lightweight because a reversible choice does not justify
  NTM, Agent Mail, or panel ceremony.

## Route before running

- A cheap, reversible choice is a two-way door. Route it to `idea-genie` or one
  fresh-context challenge. Emit the lightweight two-way-door packet and do not
  require a pane manager, messaging service, or model panel.
- A change to a public contract, port boundary, migration posture, or other
  costly-to-reverse surface is a one-way door. Run the sealed workflow below.

## Sealed workflow

1. Freeze the decision question, constraints, evidence paths, and review rubric.
2. Create at least two fresh contexts with distinct context identifiers. Each
   produces its perspective before any perspective is revealed to another.
3. Seal generation, then cross-review by rubric dimension. At minimum inspect
   evidence, reversibility, system fit, failure modes, and cost.
4. Attempt concrete refutations. Preserve disagreements, failed refutations,
   and minority reasoning; synthesis must not erase them.
5. Write `idea-challenge.v1`, validate it, and hand its artifact directory to
   `ao plan-pawl decide`. `council` may supply independent judgment mechanics,
   while the plan-pawl remains the sole decision owner.

## Output Specification

- **Artifact directory:** `.agents/duel/<run-id>/`
- **Filename convention:** `idea-challenge.json`
- **Format:** `idea-challenge.v1` JSON with the route-specific fields enforced
  by the validator.
- **Validation command:** `skills/dueling-idea-genies/scripts/validate-output.sh <idea-challenge.json>`
- **Downstream handoff:** for a one-way door, pass the validated artifact
  directory to `ao plan-pawl decide`; for a two-way door, route the packet to
  `idea-genie` for a single fresh-context challenge.

One-way-door packets contain sealed, distinct perspectives, dimensional
cross-reviews, preserved disagreements, and concrete refutation attempts.
Two-way-door packets explicitly disable sealed panel orchestration and carry
the lightweight `idea-genie` route. This skill does not write a decision
verdict in either case.

The validator is the machine boundary:

```bash
skills/dueling-idea-genies/scripts/validate-output.sh <challenge.json>
```

Executable behavior:
[references/dueling-idea-genies.feature](references/dueling-idea-genies.feature).

## Quality

- Every one-way-door packet proves perspectives used distinct context IDs and
  cross-reviewed another perspective by named dimensions.
- Dissent and attempted refutations remain explicit; synthesis never erases a
  minority position or substitutes for the plan-pawl decision.
- The named validator passes before the artifact directory is handed to its
  route owner.

## Do not

- Let perspectives see one another before sealed generation completes.
- Convert consensus, transport availability, or a self-score into a verdict.
- Require orchestration infrastructure for a reversible choice.
