---
name: cc-cron-ticks
description: "Schedule autonomous flywheel ticks inside a Claude Code session — CronCreate/CronList/CronDelete plus the schedule routine surface — for in-session drive loops, idempotent ticks, and one-shot vs recurring cadence."
---

# cc-cron-ticks (Codex)

This is the Codex-runtime entry point. The full doctrine — overview, critical
constraints, the four-phase workflow, output spec, quality rubric, examples,
troubleshooting, and currency anchor — lives in the sibling
[`../SKILL.md`](../SKILL.md). Read it first.

Codex has **no `CronCreate`/`CronList`/`CronDelete` tools and no `schedule`
surface** — the Claude-native scheduler does not exist here. The execution
profile for delivering the same outcome (a wall-clock-cadenced, idempotent drive
tick) on Codex is in [`prompt.md`](./prompt.md): use OS-level scheduling
(launchd / systemd-timer / `cron(8)`) firing a thin idempotent tick body, never
`claude -p`.
