---
name: premortem
description: Optionally challenge a frozen plan with one
---
# Premortem

Premortem is an optional plan-challenge strategy. It asks one fresh context to
identify concrete ways a frozen PlanPacket could fail before implementation.
It is not part of the required RPI sequence and does not authorize readiness.

## Workflow

1. Pin the PlanPacket digest, acceptance, non-goals, evidence requirements, and
   declared write scope.
2. Use one fresh judge with a context ID distinct from the plan author.
3. Test acceptance completeness, edge behavior, scope, dependencies,
   reversibility, and evidence shape against cited repository facts.
4. Return one complete set of concrete findings and checked/not-checked scope.
5. Stop. The caller decides whether to revise the plan or invoke RPI.

Council or Dueling Idea Genies may be caller-supplied evidence, but Premortem
does not require either strategy and cannot turn consensus into approval.

## Boundary

- Emit advisory findings, not `verdict.v2`, readiness, admission, or permission.
- Do not implement, validate the candidate, retry, repair, schedule, claim,
  change acceptance, operate Git, close work, release, or deliver.
- Any plan edit creates a new subject for a later caller-initiated Premortem.

## Output

Return `premortem-plan-review.v1` with the plan digest, author and judge context
IDs, findings, evidence references, `checked`, and `not_checked`. An empty
finding set means only that this optional challenge found no concrete defect;
it is never a lifecycle gate.
