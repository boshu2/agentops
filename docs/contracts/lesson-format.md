# Optional Lesson Format

Learn may analyze collections of durable verdicts after the operating loop has
finished. When a caller chooses to preserve a recurring pattern as a lesson, use
one Markdown file under a caller-selected directory such as
`.agents/learnings/`. Lessons are advisory evidence; they never change RPI
sequencing, a candidate identity, or a verdict.

## Required fields

```yaml
---
id: <kebab-case>
date: YYYY-MM-DD
pattern: <specific recurring behavior or defect>
evidence:
  - <verdict digest or stable artifact pointer>
scope: <where the pattern applies>
falsified_by: <observable evidence that would invalidate it>
suggested_action: <reference, skill edit, deterministic check, or none>
---
```

The body explains the observed cases, why they appear related, uncertainty, and
the smallest proposed response. Use at least two independent examples unless a
single authoritative incident is explicitly sufficient.

## Boundaries

- A lesson records an inference; it is not a linter, policy, queue item, or
  release gate.
- Promotion into a skill, reference, or deterministic check is a separate
  caller-authorized change with its own acceptance and validation.
- Do not create tracker work, install hooks, mutate CI, or start another RPI
  invocation from the lesson writer.
- Preserve source verdict digests so later readers can reproduce or refute the
  pattern.
- Archive or delete lessons that are contradicted, unactionable, or no longer
  relevant.

Search and retention belong to the caller's filesystem or knowledge tools.
AgentOps does not require a recall service, provenance ledger, or hosted store.
