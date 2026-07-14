# Failure Recovery Evidence

Crank classifies failure evidence and returns it at the wave boundary. It does
not own a retry allowance, wave cap, task budget, disposition transition, or
helper consultation. RPI's persistent governor is the sole controller.

## Validation failure handling

When swarm validation fails:

1. Preserve the failed issue identifier, runnable check, exit status, and
   smallest useful output in the wave result.
2. Record the attempted approach and any safe rollback point.
3. Return `BLOCKED` or `PARTIAL` evidence to the orchestrator without changing
   terminal tracker state or dispatching another worker.
4. Let the orchestrator classify `REPAIR`, `REPLAN`, or breaker evidence and
   request any later action through the persistent governor.

## Admission refusal

Every wave begins with a durable `crank-wave` admission. If the governor
refuses, Crank stops before dispatch and returns the refusal receipt. Crank must
not infer remaining budget, initialize a replacement run, or translate the
refusal into a helper request.

## Pre-flight: issues exist

Verify that beads mode has ready issues or TaskList mode has pending unblocked
tasks. An empty ready set is evidence, not proof of epic completion. Report
whether all children are complete, dependency-blocked, or absent, then return
to the orchestrator.

## Evidence taxonomy

| Signal | Proposed classification | Evidence to return |
|--------|-------------------------|--------------------|
| Transient transport or service error | `REPAIR` candidate | Error, affected action, and safe replay boundary |
| Partial completion or conflicting write scope | `REPLAN` candidate | Completed scope, remaining scope, and collision |
| Missing API, impossible contract, or external dependency | Breaker candidate | Blocker class, failed assumption, and required authority |
| Repeated unchanged outcome | No-progress or oscillation candidate | Approach history and identical result signature |

These are proposals only. Crank never turns a proposed classification into a
run disposition.

## Final evidence handoff

Assemble wave checkpoints, changed-file attribution, acceptance commands, and
unresolved findings for Validate. Per-wave deterministic checks do not replace
the independent verdict. The verdict flows through Learn before the
orchestrator chooses another action.

When a fresh-context helper is warranted, the governor first enters `HOLD` with
the matching blocker class. Crank does not invoke the helper itself and does
not mark human-only state. It simply preserves enough evidence for the
authority-bearing RPI command to act.
