# Goals

## Product outcome

AgentOps makes one coding-agent experiment independently inspectable without
taking over the consumer's engineering system.

The canonical outcome is:

```text
explicit behavior
  -> bounded implementation experiment
  -> exact content identity
  -> fresh independent judgment
  -> durable PASS | FAIL | NOT_PROVEN
```

## Fitness properties

1. **Behavior before activity.** Every PlanPacket contains normal and edge
   Given/When/Then scenarios, non-goals, evidence, and bounded write scope.
2. **One experiment.** Implement performs one RED -> GREEN -> refactor cycle and
   reports facts without retry or delivery authority.
3. **Fresh judgment.** PASS requires explicit, distinct author and validator
   context IDs plus a freshness attestation.
4. **Exact subject.** Content identity covers files, symlinks, deletions,
   executable bits, roots, exclusions, and canonical digests without requiring
   Git or `ao`.
5. **Honest uncertainty.** Mutation, incomplete changed-path coverage, missing
   identity, or missing proof returns NOT_PROVEN. Proven scope or acceptance
   failure returns FAIL.
6. **Sovereign proof.** Validate atomically writes content-addressed JSON to
   caller-controlled storage. Provenance is optional audit.
7. **Stop boundary.** RPI dispatches Plan, Implement, and Validate at most once,
   reports the result, and emits no next action.
8. **Open ecosystem.** Callers keep their trackers, Git, PRs, CI, cloud agents,
   merge queues, rollback, and release systems.

## Structural constraints

- Core hard dependencies are only `rpi -> {plan, implement, validate}`.
- Learn and all strategy/factory/specialist skills are off-path or optional.
- Core schemas contain no retry, budget, queue, claim, lease, admission,
  next-action, closure, release, or delivery state.
- The pure manifest and verdict helpers make no Git, tracker, queue, network,
  release, or delivery call.
- Deterministic `ao gate check` reports repository check success or failure only.
- Semantic validation is a skill responsibility, not a CLI state machine.

## Measured learning hypothesis

Collections of durable verdicts may reveal repeated defect classes that deserve
better context, tests, or checks. Learn may propose evidence-backed candidates
off-path. Promotion requires separate evaluation; no observation silently
becomes policy or changes a prior verdict.
