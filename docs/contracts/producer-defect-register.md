# Producer-Defect Recurrence Contract

This contract describes an optional, post-run reduction from repeated Validate
findings to an advisory producer-rule candidate. It does not participate in RPI,
change a verdict, create policy, block work, or activate a mechanical check.

## Inputs

Each evidence-backed finding observation conforms to
`schemas/finding-observation.v1.schema.json` and carries:

- a stable defect `class_key`;
- the `objective_id` whose work was validated;
- an observation ID and immutable `evidence_ref`;
- a concise summary of the defect.

Retries are not independent evidence. Multiple observations with the same
`class_key` and `objective_id` collapse to one occurrence, even if the diff,
commit, validator attempt, or review round changed.

## Reduction

The recurrence reducer groups by `class_key`, then counts distinct
`objective_id` values:

- one distinct objective remains an observation and emits no candidate;
- two or more distinct objectives emit exactly one advisory candidate for the
  class;
- the candidate cites one representative evidence reference per distinct
  objective and reports that distinct-objective count as `recurrence_count`.

Candidates conform to `schemas/producer-rule-candidate.v1.schema.json`. They
are always `advisory: true`. A candidate proposes that a later Learn invocation
examine Plan, Premortem, a specialist skill, or an operator footgun. It is not a
rule, gate, lifecycle transition, or instruction to revise the completed run.

## Optional projection

An explicitly invoked Learn consumer may read a caller-supplied collection of
immutable `verdict.v2` artifacts, normalize their findings into observations,
and emit this reduction. There is no required receipt, command, background
process, automatic invocation, or core-state update.

Any before/after measurement remains a separate advisory projection. Candidate
creation and effectiveness measurement must not be conflated.
