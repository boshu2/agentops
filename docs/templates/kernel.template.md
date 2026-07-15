# Project: {{PROJECT_NAME}}

## Behavioral standards

- Read the relevant source before changing it.
- State one observable behavior and a critical edge.
- Preserve unrelated work and declare the intended write scope.
- Run the smallest useful deterministic check while editing.
- Bind semantic validation to exact content and a fresh validator context.
- Report checked and unchecked scope honestly.

## AgentOps loop

```text
RPI -> Plan -> Implement -> fresh Validate -> durable verdict -> report and stop
```

The caller owns revisions, retries, work organization, Git, CI, release, and
delivery. Optional specialists, councils, and runtime adapters are invoked only
when the caller selects them.

## Project constraints

{{ADDITIONAL_CONSTRAINTS}}
