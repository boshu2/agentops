# Cron, Loops, and Automation

<!-- TOC: Four Automation Surfaces | When To Use Each | /loop Pattern | CronCreate Pattern | Shell cron | Stuck-Detection Hook | Rate-Limit Wake | Productivity Gate | Cadence Cheatsheet | Idempotency | Shutdown | Escape Hatch -->

Once you're orchestrating a swarm for ≥30 min, the cadence should be automated. Every minute you hand-tick is a minute you should be coding. This reference catalogs the four automation surfaces and when to use each.

## Four Automation Surfaces

| Surface | Ownership | Best For |
| --- | --- | --- |
| `/loop <interval> <prompt>` | Optional in-session slash tool, when available | Short-lived orchestration; dynamic pacing; re-plan on each tick |
| `CronCreate` | Remote scheduled agent (separate context) | Long-running (hours/days), off-context, survives your session ending |
| Shell `cron` (`crontab -e`) | OS-level | Pure-tooling ticks (no LLM needed): metrics, backups, cleanups |
| `/schedule` | Optional scheduling slash tool, when available | Multi-step cron with skill invocations |

## When To Use Each

- **Trying a pattern for the first time** → `/loop 10m "<prompt>"` if available — cheap, interruptible, you can watch it.
- **Steady-state 1-3 project swarm you'll tend for a day** → `CronCreate` with 15-min ticks, off your main context.
- **Metrics gathering (commits/hour, rate-limit events, disk use)** → shell cron every 5-15 min.
- **Multi-phase workflows** (gemini review → close beads → push) → `/schedule` if available, otherwise use CronCreate or shell cron with explicit phase prompts.

## `/loop` Pattern

```text
/loop 15m Tending franken_networkx swarm. One tick:
  1. Snapshot state: ntm --robot-is-working, --robot-health-oauth, --robot-quota-status, --robot-health-restart-stuck --dry-run
  2. Check convergence: git log since 1h ago + br ready count + br list --status=in_progress,claimed count
  3. Escalate stuck panes (≥3 ticks identical tail) via the OC-003 ladder
  4. Rotate rate-limited panes via OC-002
  5. Dispatch specific-terse nudges to idle panes — one concrete bead each
  6. If convergence triple-check holds (see OC-016), stop the loop
```

When `/loop` is available, dynamic mode (no interval) is ideal for deep-work phases where 30+ minute waits are natural.

## `CronCreate` Pattern

Remote agent owns the orchestration; you can close your session and it keeps going.

```text
# Prompt for CronCreate
Every 15 minutes, operate on session franken_networkx. One tick as defined in /vibing-with-ntm Operator Loop. Autonomous recovery authorized. Stop when convergence triple-check holds (git log 1h = 0 AND ready=0 AND in-flight unchanged ≥2 ticks). Report status to me via send_message on thread=swarm-daily.
```

Benefits: survives session compaction, runs in its own context, cheaper token-wise per tick.

Caveat: remote agents have stricter permissions; pre-auth `ntm rotate`, `ntm restart-pane` etc. on the account before scheduling.

## Shell `cron` For Pure Tooling

Nothing fancy, no LLM invocation — just data collection:

```bash
# ~/.crontab for /tmp/tick_metrics.sh — updates every 5 min
*/5 * * * * /usr/local/bin/tick_metrics.sh >> /var/log/swarm_metrics.log 2>&1

# /usr/local/bin/tick_metrics.sh
#!/bin/bash
set -euo pipefail
REPO=/data/projects/franken_networkx
TS=$(date -u +%FT%TZ)

commits_1h=$(git -C "$REPO" log --since="1 hour ago" --oneline | wc -l)
ready=$(cd "$REPO" && br ready --json | jq '.issues | length' 2>/dev/null || echo -1)
inflight=$(cd "$REPO" && br list --status=in_progress,claimed --json | jq '.issues | length' 2>/dev/null || echo -1)
disk_pct=$(df /data | tail -1 | awk '{print $5}' | tr -d '%')

echo "${TS} commits_1h=${commits_1h} ready=${ready} inflight=${inflight} disk_pct=${disk_pct}"
```

You can then grep / plot this log to detect productivity drops long before the swarm feels stuck.

## Stuck-Detection Automation (cc-hooks integration)

If you want stuck panes to auto-escalate without any orchestrator polling at all, wire up a PostToolUse / background hook. See the `/cc-hooks` skill.

Sketch (conceptual; check `/cc-hooks` for current API):

```json
{
  "hooks": {
    "PeriodicCheck": [
      {
        "minutes": 15,
        "command": "ntm --robot-health-restart-stuck=$CURRENT_SESSION --stuck-threshold=10m --dry-run | jq -e '.stuck_panes | length > 0'",
        "onMatch": "ntm --robot-health-restart-stuck=$CURRENT_SESSION --stuck-threshold=10m"
      }
    ]
  }
}
```

This turns the stuck-detect rung of OC-003 into a zero-attention background process.

## Rate-Limit Wake Automation

Don't poll rate limits — use `--robot-wait` so ntm wakes you when the wall drops.

```bash
# In a background bash task or cron agent
ntm --robot-wait=<session> --wait-until=rate_limited --timeout=30m && \
  ntm --robot-send=<session> --msg="Rate limit lifted. Resume per domain assignment." && \
  logger -t swarm-auto "Woke from rate-limit wait on <session>"
```

When `rate_limited` condition becomes false for any pane in the session, the wait returns. Re-dispatch and continue.

## Productivity Gate Automation

Auto-exit the orchestrator loop when the swarm converges. Add this to every cron-ticked recipe:

```bash
#!/bin/bash
# convergence-check.sh — exits 0 if swarm should continue, 1 if converged
set -euo pipefail
REPO="$1"
PREV_INFLIGHT_FILE="/tmp/swarm-prev-inflight-$(basename "$REPO")"

commits_1h=$(git -C "$REPO" log --since="1 hour ago" --oneline | wc -l)
ready=$(cd "$REPO" && br ready --json | jq '.issues | length')
inflight=$(cd "$REPO" && br list --status=in_progress,claimed --json | jq '.issues | length')
prev_inflight=$(cat "$PREV_INFLIGHT_FILE" 2>/dev/null || echo -1)

# Persist
echo "$inflight" > "$PREV_INFLIGHT_FILE"

# Converged?
if [ "$commits_1h" -eq 0 ] && [ "$ready" -eq 0 ] && [ "$inflight" -eq "$prev_inflight" ]; then
  echo "CONVERGED: commits=0 ready=0 inflight=$inflight unchanged"
  exit 1
fi

exit 0
```

Wrap every tick:

```bash
*/15 * * * * /usr/local/bin/convergence-check.sh /data/projects/foo && /usr/local/bin/swarm-tick.sh /data/projects/foo
```

If convergence-check exits 1, the tick script doesn't run. Self-healing loop.

## Cadence Cheatsheet

| Phase | Poll interval | Cost |
| --- | --- | --- |
| Just dispatched (watching for nucleation) | 4 min | High (watch closely) |
| Panes deep in build/test | 15-20 min | Low (rch builds are 5-10 min) |
| Steady review/close work | 10-15 min | Medium |
| Overnight deep-work | 30-60 min | Very low |
| Convergence suspected | 30 min | Minimal |

Never sub-3-min poll. Never hand-tick >30 min.

## Idempotency Rule

Every tick must be safe to run twice. If a dispatch arrived twice, the agent should recognize duplication and no-op. Design your nudge prompts accordingly ("If you already claimed this bead, ignore; otherwise claim and start.").

## Shutdown Automation

When you want to cleanly end a swarm (after convergence, before the merge freeze, etc.):

```bash
# 1. Broadcast graceful shutdown prompt
ntm send <session> --all --skip-first --no-cass-check "Finish your smallest coherent piece of work. Checkpoint status. Update bead status and coordination thread. Then stop cleanly."

# 2. Wait for all agents to idle
ntm --robot-wait=<session> --wait-until=idle --timeout=20m

# 3. Snapshot final state
ntm --robot-markdown --md-compact > "/tmp/swarm-$(date +%Y%m%d-%H%M)-final.md"

# 4. Push everything
cd "$REPO"
BEADS_DIR="$(ao beads dir)" br sync --flush-only
git -C "$(ao beads dir)" add -A
git -C "$(ao beads dir)" commit -m "sync beads" || true
git -C "$(ao beads dir)" push
git push

# 5. Kill the session (confirm first)
ntm kill <session>
```

Wire this into a cron that fires on convergence detection.

## Escape Hatch

If any automated loop misbehaves (spamming a pane, infinite restart-loop, wrong session), kill it fast:

```bash
# For /loop running in-session: /loop stop
# For CronCreate agents: CronList → CronDelete <id>
# For shell cron: crontab -l | grep -v <pattern> | crontab -
# For wait processes: pkill -f "robot-wait"
```

Always know how to stop what you started. Every automation surface has a kill command; pre-learn it before starting.
