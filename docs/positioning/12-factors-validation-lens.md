# The 12 Factors as a Supporting Lens for AgentOps

The original 12-factor doctrine still matters here, but only as supporting doctrine. The primary product definition lives in [PRODUCT.md](https://github.com/boshu2/agentops/blob/main/PRODUCT.md), [README.md](https://github.com/boshu2/agentops/blob/main/README.md), [docs/context-lifecycle.md](../context-lifecycle.md), and [docs/software-factory.md](../software-factory.md).

Publicly, AgentOps is the **operational layer for coding agents**. Under the
hood, it behaves like a **software-factory control plane**. The environment
carries continuity, the workers remain replaceable, and the system proves
itself through an internal lifecycle contract: validation, bookkeeping, and
closure. This is the 12-factor expression of environment-carried continuity
with replaceable workers. This lens explains why the 12-factor shape still
fits, but it does not replace the current product framing.

## How to Read This Lens

The 12-factor model is useful because it describes an operating style where state belongs in the environment, processes are disposable, and the control plane stays explicit. AgentOps uses that same shape, but the goal is different:

- load the right context before work starts
- validate work before code ships
- retain what the repo learned through bookkeeping
- close the loop so the next session starts smarter

That is the operating logic behind the software-factory control plane described in the repo docs.

## The Twelve Factors

### Factor I: Codebase

One codebase still matters, but in AgentOps the codebase is only the durable substrate for the control plane. The repo holds the workflows, hooks, contracts, docs, and artifacts that make validation and learning repeatable.

The point is not "one repo because 12-factor says so." The point is one shared contract that every worker reads from and writes back to.

### Factor II: Dependencies

Dependencies must be explicit because validation depends on reproducibility. In AgentOps that includes CLI commands, hooks, schemas, skill contracts, and the supporting documents that define how the control plane behaves.

If a worker cannot declare its dependencies, the environment cannot validate
its work or reproduce its result.

### Factor III: Config

Configuration is environment-carried context: briefings, goals, constraints, risks, and phase-specific packets. This is where the repo keeps the context that would otherwise disappear when a session ends.

AgentOps treats config as part of the operating environment, not as ad hoc prompt stuffing. That is how the environment carries continuity with replaceable workers.

### Factor IV: Backing Services

Backing services are attached to the control plane, not embedded inside the worker. In AgentOps, that includes knowledge retrieval, issue tracking, validation gates, and other repo-native services that support the session without becoming the session.

The worker can be swapped out. The service contract remains.

### Factor V: Build, Release, Run

AgentOps separates discovery, planning, implementation, validation, and learning so the repo can judge work at the right time. That is the same discipline the 12-factor model wanted from build, release, and run, but applied to the agent loop.

The environment should know when work is still being shaped, when it is ready
to validate, and when it has become bookkeeping that future sessions can use.

### Factor VI: Processes

Workers are processes, not permanent identities. They should be replaceable, bounded, and restartable. The environment is what persists.

This is the clearest expression of the software-factory control plane: workers
come and go, but the repo keeps the state that matters for validation,
bookkeeping, and closure.

### Factor VII: Port Binding

AgentOps binds the operator surface explicitly. Commands such as `/rpi`, `/pre-mortem`, `/vibe`, `/retro`, and `ao factory start` are not hidden internals; they are the ports through which the control plane is exercised.

The value of explicit binding is not just convenience. It makes the operator surface legible enough to validate, extend, and hand off.

### Factor VIII: Concurrency

Concurrency in AgentOps is about isolated work lanes, not shared chat noise. Parallel workers can move independently as long as ownership is clear and the environment preserves the continuity they need.

That is why the repo emphasizes scoped execution, fresh context, and non-overlapping work rather than giant undifferentiated swarms.

### Factor IX: Disposability

Workers should be disposable; the environment should not be. Sessions can end, compactions can happen, and workers can be replaced without losing the state of the system.

This is where closure becomes operational: completed work does not vanish with
the worker. It is harvested into artifacts, findings, and follow-on work that
the next session can use.

### Factor X: Dev/Prod Parity

Parity in AgentOps means the same contract should hold from briefed session to phased run to validation and learning. The repo should not rely on a special hidden mode to stay correct.

If the control plane works in one session but falls apart in another, the environment is not actually carrying continuity.

### Factor XI: Logs

Logs are not just debugging output. In AgentOps they are the evidence trail for
validation, the bookkeeping substrate for durable learning, and the raw
material for closure.

That includes transcripts, findings, learnings, constraints, and the artifacts that make later validation stronger than earlier validation.

### Factor XII: Admin Processes

Admin work should be first-class and repeatable, not a hidden manual ritual. Maintenance commands, flywheel routines, curation workflows, and goal checks exist so the control plane can keep itself healthy.

This is where AgentOps turns the 12-factor idea into durable practice: the environment maintains itself, learns from itself, and keeps the next session oriented toward better work.

## What the Lens Emphasizes

The 12-factor shape is still useful because it reinforces the repo's actual operating model:

| Lens | AgentOps meaning |
|------|------------------|
| Config and backing services live outside the worker | The environment carries continuity |
| Processes are disposable | Workers remain replaceable |
| Build/release/run are separated | Validation happens before the cost of shipping |
| Logs and admin processes are first-class | Bookkeeping and closure are explicit |

The doctrine is therefore supportive, not defining. The product is not
"12-factor agents." The product is the operational layer for coding agents,
with a software-factory control plane underneath it.

## Canonical Contract Mapping

- **Validation** maps internally to judgment validation: the parts of the
  system that challenge the plan and the code before ship time.
- **Bookkeeping** maps internally to durable learning: the parts of the system
  that extract, store, curate, and retrieve what the repo learned.
- **Closure** maps internally to loop closure: the parts of the system that
  turn completed work into the next work, the next rule, and the next context
  packet.
- **Software-factory control plane** is the umbrella: the environment carries continuity, workers are replaceable, and the operator surface stays explicit.

## Bottom Line

12-factor language still helps explain AgentOps, but only as a lens. The real
product story is the operational layer for coding agents. The internal
contract proves that story by validating work, preserving bookkeeping, and
closing the loop inside a software-factory control plane where the environment
carries continuity and workers remain replaceable.
