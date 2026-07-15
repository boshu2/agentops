# Getting started

Install or expose the AgentOps skills to your agent runtime, then invoke RPI with
one behavior:

```text
/rpi Add an edge-case-safe parser for the supplied format.
```

The expected output is a PlanPacket, CandidatePacket, exact subject manifest,
fresh independent verdict, and final RPI report. The loop stops after reporting.

For direct use, invoke Plan, Implement, or Validate separately and supply their
declared packet inputs yourself. See the [skill router](../SKILL-ROUTER.md) and
[first value path](../first-value-path.md).
