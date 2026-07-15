# Optional Learning Loop

AgentOps core ends when Validate writes a durable verdict and RPI reports it.
Learning is deliberately off the critical path.

When a caller wants longitudinal analysis, it may explicitly invoke Learn over
a collection of immutable `verdict.v2` artifacts. Learn can group concrete
finding observations, identify recurrence across distinct objectives, and
suggest an advisory producer-rule candidate.

Learn does not:

- change a completed verdict;
- repair or re-plan work;
- choose a next invocation;
- activate a rule or deterministic check;
- mutate Git, a tracker, or delivery state; or
- block RPI when its own storage or analysis is unavailable.

Promotion from an observed pattern into a skill, test, or repository rule is a
separate caller-authorized change with its own Plan, Implement, and Validate
cycle. This preserves the useful compounding idea without making bookkeeping a
condition for finishing ordinary work.

See [the producer-defect recurrence contract](contracts/producer-defect-register.md).
