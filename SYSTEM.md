# AgentOps Dynamo System

AgentOps is a self-exciting dynamo for agentic work: fungible worker tokens
rotate through a deterministic control loop, the loop's field filters variance
through gates and ratchets, accepted work is the current, and measured corpus
delta feeds back as self-excitation.

This file names one loop that already exists across the repository and bead
graph. It is the system map for `ag-w4r7c`, not a new subsystem.

## Dynamo Map

| Dynamo term | AgentOps meaning | Existing organ | System signal |
| --- | --- | --- | --- |
| Rotor | Fungible worker tokens: any eligible agent, runtime, host, or lane that can execute the work and return evidence. | Runtime dispatch, worktrees, Agent Mail coordination, bead claims. | Attempted work, usage, rework. |
| Field | The operating loop plus the pawl gate plus the ratchet. | `docs/contracts/pawls.md`, `scripts/pawl-verdict.sh`, local gates, accepted bead policy. | Claim accepted, rejected, held, or sent back around the loop. |
| Current | Accepted work only: evidence-backed work that passed the pawl gate and reached the accepted state. | Merge/accept events and bead state. | `yieldledger.EventAccept`, accepted commit, accepted bead. |
| Self-excitation | Corpus delta: the measured improvement from using the accumulated AgentOps corpus. | `ag-8p8o` and its A/B experiment line. | `C` in the yield vector; pending until measured, never fabricated. |
| Sensors | Durable bead-keyed operational events. | `cli/internal/yieldledger` and `.agents/yield/yield-ledger.jsonl`. | `accept`, `gate-verdict`, and `usage` events; gauges `A`, `R`, `A/R`, `Q`, `E`, `L`, `C`. |
| Controller | Reconcile-by-rejection over workload objects. | `ag-v1xk`: goal plus acceptance contract dispatches until admitted or rejected. | Next action, retry, hold, or terminal accept. |
| Structural gate | Separation of duties and fail-closed launch authority. | `ag-xdrw` for author-not-judge; `ag-zqqm` for explicit approval before factory on-switch. | No self-approval, no unapproved irreversible launch. |

## Control Loop

1. A bead states a goal and acceptance contract.
2. The controller (`ag-v1xk`) dispatches one or more fungible workers against
   that contract.
3. A worker produces a claim, implementation, and evidence. The worker family is
   not the system boundary; any capable worker can do the work.
4. The structural gate (`ag-xdrw`) prevents self-approval before the work can
   become accepted. The factory on-switch gate (`ag-zqqm`) fails closed when
   explicit launch authority is absent.
5. At the pawl, the review gate requires both fresh-context reviewer families:
   one Claude reviewer and one Codex reviewer. Both must agree on the live
   commit-bound evidence. AGY/Gemini is not a reviewer for this gate.
6. Confirmed work becomes current: the accepted work is merged or otherwise
   recorded as accepted by the orchestrator that owns the door.
7. The yield ledger records the operational facts: `gate-verdict`, `usage`, and
   terminal `accept`. These events are sensors, not authorities.
8. Gauges compute yield from data:
   `A` accepted work, `R` raw spend, `A/R` watch-only efficiency, `Q` clean
   first-pass yield, `E` escalation or hold rate, `L` loss, and `C` corpus
   delta.
9. Assay and corpus machinery mine accepted evidence and verdicts into corpus
   updates. If `C` becomes positive under `ag-8p8o`, the stronger corpus feeds
   the next rotation and increases the field.

## Invariants

- Execution is fungible. The system must not encode a Codex-only executor rule,
  a Claude-only executor rule, or a host-specific "no Claude executor on Mac"
  rule. Runtime availability and broken transports are routing facts, not
  eligibility doctrine.
- Review is not fungible at the pawl. The current acceptance gate is a
  fresh-context Claude reviewer plus a fresh-context Codex reviewer, and both
  must agree. A single fresh-context review, AGY, Gemini, or author self-review
  does not satisfy the gate.
- The author cannot be the judge. `ag-xdrw` is the structural separation of
  duties: work can be produced by any lane, but accepted only after non-author
  review.
- Accepted work is the only current. Attempts, drafts, green local checks, and
  unsupported claims are rotor motion until the gate admits them.
- The yield ledger is a sensor. It is fail-open observability and must not block
  the action it observes; authority stays in the pawl gate and orchestrator-owned
  merge door.
- `C` is measured, not asserted. Until `ag-8p8o` publishes a reproducible corpus
  delta, `C` remains pending.
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
  -> Claude + Codex fresh-context pawl
  -> accepted work current
  -> yield-ledger sensors
  -> corpus delta assay
  -> stronger next rotation
```

The product claim is therefore operational: stochastic work is allowed to vary,
but only evidence-backed, cross-reviewed, accepted work contributes current, and
only measured corpus delta can claim self-excitation.
