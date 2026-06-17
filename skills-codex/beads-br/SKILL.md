---
name: beads-br
description: "Run beads br."
---

# beads-br (Codex)

Codex-native entry point for the `beads-br` operator skill.

The AgentOps source skill `../../skills/beads-br/SKILL.md` is the source of truth
for domain behavior, commands, examples, references, and output expectations.
Read it first, then use `prompt.md` for the Codex runtime profile.

## Codex Runtime Contract

- Use Codex plus the local shell. Do not invoke Claude Code as an executor.
- Load only the relevant source references or scripts for the task.
- Prefer robot/JSON/NDJSON command surfaces when the source skill exposes them.
- Verify command syntax from local `--help` or checked-in references before acting.
- Return concrete evidence: commands run, files touched, exit codes, and any remaining blocker.

## Issue-Lifecycle Discipline (folded from retired `beads`)

The source skill now carries the issue-lifecycle doctrine folded from the retired
`beads` umbrella (ag-ez7y6). Honor it from Codex too:

- Treat live `br show` / `br ready` / `br list` as authoritative; the exported
  `issues.jsonl` is a git-friendly artifact, not the primary decision source.
- Every `br close` carries scoped closure proof: touched files, validation
  command(s) run, and the parent-reconciliation outcome — never a bare "done".
- Reconcile the open parent in the same session; narrow a broad umbrella bead into
  an execution-ready child before implementing; normalize stale queue items rather
  than skipping them.
