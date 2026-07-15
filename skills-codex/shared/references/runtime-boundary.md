# Runtime boundary

Runtime adapters may start or observe explicitly requested sessions. Their
provider-specific attempts, reconnects, queues, and completion states remain
adapter evidence and cannot alter core Plan, Candidate, RPI, or verdict state.
Only Validate writes `verdict.v2`.
