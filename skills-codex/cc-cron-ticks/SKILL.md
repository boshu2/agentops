---
name: cc-cron-ticks
description: |
  Schedule autonomous flywheel ticks inside a Claude Code session — CronCreate/CronList/CronDelete plus the /schedule routine surface — for in-session drive loops, idempotent ticks, and one-shot vs recurring cadence.
  Triggers: "schedule a tick", "drive the flywheel", "every N minutes run", "1-min loop", "in-session cron", "one-shot reminder", "recurring routine", "CronCreate", "CronList", "CronDelete", "/schedule", "cron job for claude", "re-fire a prompt on a cadence".

  **Use when:**
  - You need an autonomous loop binary/harness re-invoked on a wall-clock cadence while the REPL is idle.
  - You need a one-shot future action ("at 3pm run the smoke test") that fires once then deletes itself.

  **Perfect for:**
  - The 1-minute in-session drive loop that ticks an evolve/factory/control-plane harness.
  - Cadence design for periodic burndown, health checks, or queue draining.

  **Not ideal for:**
  - Watching a log/process/command output for the moment it changes (use Monitor — cron polls, Monitor streams).
  - Cross-machine durable scheduling that must survive a host reboot (use launchd/systemd/cron(8)).
---

# cc-cron-ticks (Codex)

This is the Codex-runtime entry point. The full doctrine — overview, critical
constraints, the four-phase workflow, output spec, quality rubric, examples,
troubleshooting, and currency anchor — lives in the sibling
[`../SKILL.md`](../SKILL.md). Read it first.

Codex has **no `CronCreate`/`CronList`/`CronDelete` tools and no `/schedule`
surface** — the Claude-native scheduler does not exist here. The execution
profile for delivering the same outcome (a wall-clock-cadenced, idempotent drive
tick) on Codex is in [`prompt.md`](./prompt.md): use OS-level scheduling
(launchd / systemd-timer / `cron(8)`) firing a thin idempotent tick body, never
`claude -p`.
