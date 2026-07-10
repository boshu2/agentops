---
name: dueling-idea-genies
description: Challenge a contested one-way-door idea with
---
# Dueling Idea Genies

Produce independent challenges for a consequential choice. The output is
evidence for the existing plan-pawl decider; this skill never assigns
PASS/REDO/BLOCKED and never counts the author as an independent perspective.

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

```bash
skills/dueling-idea-genies/scripts/validate-output.sh <challenge.json>
```

Executable behavior:
[references/dueling-idea-genies.feature](references/dueling-idea-genies.feature).

## Do not

- Let perspectives see one another before sealed generation completes.
- Convert consensus, transport availability, or a self-score into a verdict.
- Require orchestration infrastructure for a reversible choice.
