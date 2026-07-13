# Producer-Defect Recurrence Contract

This contract governs the bookkeeping seam from repeated validation findings to
an advisory producer-rule candidate. It does not create policy, block delivery,
or activate a mechanical check.

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
are always `advisory: true`. A candidate proposes that the orchestrator examine
the producer surface—Discovery, Plan, Premortem, a skill contract, or an
operator footgun—but it is not itself a rule or gate.

## Runtime projection

`ao membrane digest --json` exposes the reduction as
`producer_candidates`. Learn records the same shape in its receipt after
reconciling immutable Validate observations. The orchestrator alone decides
whether a candidate changes future work.

The measured before/after register remains a separate projection: it evaluates
whether an accepted producer change reduced later recurrence. Candidate
creation and effectiveness measurement must not be conflated.
