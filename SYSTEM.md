# AgentOps Dynamo System

AgentOps is a dynamo built to test self-excitation in agentic work: fungible
worker tokens rotate through a deterministic control loop, the loop's field
filters variance through gates and ratchets, accepted work is the current, and
measured corpus delta is the feedback hypothesis.

This file names one loop that already exists across the repository and bead
graph. It is the system map for `ag-w4r7c`, not a new subsystem.

## Dynamo Map

| Dynamo term | AgentOps meaning | Existing organ | System signal |
| --- | --- | --- | --- |
| Rotor | Fungible worker tokens: any eligible agent, runtime, host, or lane that can execute the work and return evidence. | Runtime dispatch, worktrees, Agent Mail coordination, bead claims. | Attempted work, usage, rework. |
| Field | The operating loop plus the pawl gate plus the ratchet. | `docs/contracts/pawls.md`, `scripts/pawl-verdict.sh`, local gates, accepted bead policy. | Claim accepted, rejected, held, or sent back around the loop. |
| Current | Accepted work only: evidence-backed work that passed the pawl gate and reached the accepted state. | Merge/accept events and bead state. | `yieldledger.EventAccept`, accepted commit, accepted bead. |
| Self-excitation | The goal that corpus delta becomes positive: measured improvement from using the accumulated AgentOps corpus. | `ag-8p8o` and its A/B experiment line. | `C` in the yield vector; pending until measured, never fabricated. |
| Sensors | Durable bead-keyed operational events. | `cli/internal/yieldledger` and `.agents/yield/yield-ledger.jsonl`. | `accept`, `gate-verdict`, and `usage` events; gauges `A`, `R`, `A/R`, `Q`, `E`, `L`, `C`. |
| Controller | Planned reconcile-by-rejection over workload objects. | `ag-v1xk`: blocked on ratified workload/watch objects; intended shape is goal plus acceptance contract until admitted or rejected. | Future next action, retry, hold, or terminal accept. |
| Structural gate | Separation of duties and fail-closed launch authority. | `ag-xdrw` for author-not-judge; merged `ag-zqqm` for explicit approval before factory on-switch. | No self-approval, no unapproved irreversible launch. |

## Control Loop

1. A bead states a goal and acceptance contract.
2. Today, the substrate and operator dispatch one or more fungible workers
   against that contract. The planned controller (`ag-v1xk`) will make this a
   declarative reconcile loop after the workload/watch object model is ratified.
3. A worker produces a claim, implementation, and evidence. The worker family is
   not the system boundary; any capable worker can do the work.
4. The structural gate (`ag-xdrw`) prevents self-approval before the work can
   become accepted. The factory on-switch gate (`ag-zqqm`) fails closed when
   explicit launch authority is absent.
5. At the pawl, the default review gate is fresh-context: at least one
   non-author reviewer in a separate context must confirm the live commit-bound
   evidence. This default is model-agnostic; a fresh same-model reviewer
   satisfies it. A pawl can be opted up to multi-model only for the
   highest-irreversibility doors, where the requirement becomes at least two
   distinct model families, such as Claude plus Codex.
6. Confirmed work becomes current: the accepted work is merged or otherwise
   recorded as accepted by the orchestrator that owns the door.
7. The yield ledger records the operational facts: `gate-verdict`, `usage`, and
   terminal `accept`. These events are sensors, not authorities.
8. Gauges compute yield from data:
   `A` accepted work, `R` raw spend, `A/R` watch-only efficiency, `Q` clean
   first-pass yield, `E` escalation or hold rate, `L` loss, and `C` corpus
   delta.
9. Assay and corpus machinery mine accepted evidence and verdicts into corpus
   updates. If `C` is proven positive under `ag-8p8o`, the stronger corpus feeds
   the next rotation and increases the field. Until then, self-excitation is the
   dynamo hypothesis, not an achieved property.

## Invariants

- Execution is fungible. The system must not encode a Codex-only executor rule,
  a Claude-only executor rule, or a host-specific "no Claude executor on Mac"
  rule. Runtime availability and broken transports are routing facts, not
  eligibility doctrine.
- Review is not self-approval at the pawl. By default, the current acceptance
  gate is fresh-context and model-agnostic: at least one non-author reviewer in a
  separate context must confirm. Multi-model review is opt-in per pawl and
  requires at least two distinct model families when selected. Author
  self-review does not satisfy any mode.
- The author cannot be the judge. `ag-xdrw` is the structural separation of
  duties: work can be produced by any lane, but accepted only after non-author
  review.
- Accepted work is the only current. Attempts, drafts, green local checks, and
  unsupported claims are rotor motion until the gate admits them.
- The yield ledger is a sensor. It is fail-open observability and must not block
  the action it observes; authority stays in the pawl gate and orchestrator-owned
  merge door.
- `C` is measured, not asserted. Until `ag-8p8o` publishes a reproducible corpus
  delta, `C` remains pending and self-excitation remains unproven.
- The ratchet is in-repo. The field is the visible combination of loop shape,
  gate, evidence, and accepted artifact; it is not an invisible daemon or a
  model's private memory.

## Why This Matters

Without the dynamo map, AgentOps looks like scattered organs: a controller bead,
a gate bead, a corpus-delta bead, a ledger package, and a pile of workflow
rules. With the map, they become one measured loop:

```text
goal + acceptance
  -> fungible worker rotation
  -> evidence-backed claim
  -> fresh-context pawl
  -> accepted work current
  -> yield-ledger sensors
  -> corpus delta assay
  -> stronger next rotation
```

The product claim is therefore operational: stochastic work is allowed to vary,
but only evidence-backed, reviewed, accepted work contributes current, and only
measured corpus delta can claim self-excitation.
