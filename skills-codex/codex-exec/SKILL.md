---
name: codex-exec
description: "Run codex exec."
---

# codex-exec (Codex)

Codex-native parity wrapper. The full skill content — overview, critical
constraints, the three-phase workflow, output spec, exit codes, quality rubric,
examples, and troubleshooting — lives in the sibling base file `../SKILL.md`.
Read it first.

Sandbox note: offline validators use `-s read-only`; **network-touching validators**
(`git fetch`, clone, API) need `-s danger-full-access` on an already-sandboxed host —
`-s read-only` FALSE-FAILs on blocked `connect` syscalls.

Codex execution steps and guardrails for this skill are in `prompt.md` (same dir).
