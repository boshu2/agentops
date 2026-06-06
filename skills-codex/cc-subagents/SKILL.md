---
name: cc-subagents
description: |
  Drive fungible parallel workers via subscription-billed agent dispatch — background, worktree isolation, per-role model/tools/effort scoping, and an evidence-gated return. Codex-native parity of the Claude-native cc-subagents.

  Triggers: "spawn a subagent", "fan out workers", "parallel agents", "background teammate", "worktree isolation", "interchangeable workers", "spawn vs inline", "fan-out codex workers".

  Use when:
  - You need to parallelize independent, file-scoped work across interchangeable workers
  - You are deciding whether to spawn a worker vs. do the work inline in the current context

  Perfect for: fan-out of N independent tasks (per-file audits, per-module refactors, per-bead implementations); long-running background explorers/judges that must not block the orchestrator; write-heavy parallel work that must not clobber a shared tree.
  Not ideal for: a single sequential task with no parallelism (inline it); tightly-coupled edits to one file by multiple workers (combine into one worker).
---

# cc-subagents (Codex)

Codex-native parity wrapper. The full skill content — overview, the five critical
constraints (no per-token API billing, non-overlapping file ownership, fresh
context per worker, least-tools/read-only by default, evidence-gated "done"), the
four-phase workflow (decide → pick role profile → spawn → SubagentStop/collect),
the role-profile table, output spec, quality rubric, examples, and
troubleshooting — lives in the sibling base file `../SKILL.md`. Read it first.

Codex execution steps and guardrails for this skill are in `prompt.md` (same dir).
