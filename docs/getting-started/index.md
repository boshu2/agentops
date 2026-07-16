# Getting started

Install or expose the AgentOps skills to your agent runtime, then invoke RPI with
one behavior:

```text
/rpi Add an edge-case-safe parser for the supplied format.
```

The expected evidence is the exact resolved intent source, a runtime-derived
subject manifest and check receipts, one fresh independent verdict, and the
final RPI report. The loop stops after reporting.

For direct use, invoke Plan, Implement, or Validate separately. Keep the same
resolved intent bytes across phases, let the runtime derive subject identity and
changed paths, and give Validate the exact subject facts and check receipts. See
the [skill router](../SKILL-ROUTER.md) and [first value
path](../first-value-path.md).
