# Claim one transport bead

`gc agentops claim` invokes the deployment-pinned absolute `GC_BIN hook --claim
--drain-ack --json` exactly once. It accepts only the exact `{action,reason}`
hook schema, returns normalized `assigned` or `drain/no_work`, and fails closed
as `uncertain` for every nonzero, malformed, or ambiguous result. It never
retries or selects another bead.
