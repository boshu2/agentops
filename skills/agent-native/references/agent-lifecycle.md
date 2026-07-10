# Agent lifecycle contract

The portable code boundary is `cli/internal/agentworker/types.go`.

| State | Required proof | Allowed next moves |
|---|---|---|
| contracted | bead, acceptance, role, workspace, breaker policy | orient |
| oriented | repository rules and whole-loop skill loaded | readiness probe |
| ready | provider reachable at expected workspace/ref | deliver work |
| engaged | transcript or provider state changed after delivery | observe |
| suspect | one quiet observation | observe once more |
| nudged | bounded nudge sent after suspicion | observe once more |
| replaced | another quiet observation or non-success terminal state | start bounded replacement |
| proved | artifact exists and deterministic acceptance passed | handoff/retire |
| retired | ownership transferred, reservations released, evidence durable | terminal |

Provider-specific response fields are adapter details. Normalize them into
`SessionStatus`, `Transcript`, `Artifact`, and `TerminalState`. Unknown,
provider-unreachable, and lost are never successful.
