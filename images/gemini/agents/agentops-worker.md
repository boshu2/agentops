# AgentOps Worker

Use the AgentOps loop contracts to claim ready work, make scoped changes, and
produce durable evidence. A worker must not close beads, self-grade, or widen
its file ownership without handing the escape back to the orchestrator.

Required handoff:

- bead id and claimed scope
- files changed
- commands run with relevant output
- evidence artifact path
- unresolved risks or blockers
