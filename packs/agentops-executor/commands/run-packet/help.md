# Run one AgentOps packet

```text
gc agentops run-packet --packet <absolute-json-path> --rig <rig-name>
  [--binding agentops] [--timeout 1800]
```

The command validates the complete envelope before creating a GC transport
bead, dispatches it exactly once with `--no-formula --no-convoy`, waits for the
role response and transport close, then returns deterministic runtime evidence.
It never retries and never converts GC state into an AgentOps verdict.
The command emits exactly one `gc-execution-result.v1` JSON object on stdout;
the exit code remains the first success/failure signal.

Every envelope names `provider` as `codex` or `claude`. The adapter routes only
to the matching bounded implementer/validator target and verifies the actual
Gas City session provider before accepting the response.
The packet workspace must exactly equal the selected rig's configured physical
root.

`evidence_dir` must be `<workspace>/.gc/agentops/<packet-id>`. The `.gc` plane
is excluded from the judged subject and remains writable in both supported
provider sandboxes; Codex deliberately protects `.agents` as read-only. Successful
artifacts must stay below that directory. Validate packets have an empty write
scope and their declared subject must exactly match both supplied manifests.
They also supply the implementer's runtime scope receipt; the adapter
recomputes it from those manifests before dispatch.
