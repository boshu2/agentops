> **Status:** olympus v4 reference, NOT canonical for agentopsd. This file is a verbatim port from olympus/docs/specs/v4/ for cross-reference only. Where this disagrees with agentopsd's canonical design at `.agents/design/2026-04-28-design-agentops-daemon-gascity-vertical-slices.md`, **agentopsd canonical wins**.

# Daemon — Phase 0 (Mechanical)

**Status:** Draft
**Depends On:** `cli.md`

---

## What Phase 0 Is

A poll loop that spawns subprocesses. Nothing more.

The daemon discovers active quests, calls `ol zeus step` for each, reads the exit code, and sleeps. It makes **zero LLM calls and zero Anthropic API calls**. All intelligence lives in Zeus, skills, and the Claude sessions that `ol zeus step` spawns internally.

If you can write a bash `while` loop with `sleep`, you understand the daemon.

---

## Config

`~/.olympus/config.yaml`:

```yaml
workspace:
  root: "~/gt/olympus"

rigs:
  - name: olympus
    path: "~/gt/olympus"
    prefix: ol
    git: "github.com:boshu2/olympus"
  - name: agentops
    path: "~/gt/agentops"
    prefix: ag
    git: "github.com:boshu2/agentops"

daemon:
  poll_interval: 60              # seconds, minimum 30
  subprocess_timeout: 900        # 15 minutes default
  max_concurrent: 1              # 1=sequential, N>1=parallel goroutine dispatch
  fitness_interval: 300          # seconds (0=disabled, min 60 when enabled)
  fitness_threshold: 1           # regressed goals before spawning maintenance quest
  analysis_interval: 600         # seconds (0=disabled, min 60 when enabled)
  analysis_window: 100           # events to analyze for patterns
  improvement_threshold: 3       # patterns detected before spawning improvement quest
  heartbeat_timeout: 300         # seconds (0=disabled); stale team coordinator threshold

runtime:
  agent_runtime: claude          # runtime adapter family
  memrl_mode: off                # off|observe|enforce (default off)
```

**Validation rules:** `poll_interval >= 30`, `subprocess_timeout > 0`, `max_concurrent >= 1`, `fitness_interval: 0 or >= 60`, `fitness_threshold >= 1`, `analysis_interval: 0 or >= 60`, `heartbeat_timeout: 0 or >= 60`.

`memrl_mode` values are constrained to `off|observe|enforce`; invalid values normalize to `off`.

**Override priority:** CLI flags > config.yaml > DefaultConfig().

---

## Apollo Poll Loop

```
loop:
  bids   = discover_and_process_bids()          # Check bid queue FIRST
  quests = discover_active_quests()             # bd list + quest state
  ready  = select_ready(quests, max_concurrent) # FIFO, up to max_concurrent
  if no ready quests:
    check_fitness_if_idle()                     # Autonomous health monitoring
    analyze_run_ledger()                        # Pattern detection
    return
  if max_concurrent == 1:
    execute_synchronously(ready[0])             # Phase 0 mode: one at a time
  else:
    dispatch_concurrent(ready)                  # Phase 1 mode: goroutines + semaphore
      for each quest in ready (up to capacity):
        goroutine: pid = spawn("ol zeus step --quest <id>")
                   exit_code = wait(pid, timeout=subprocess_timeout)
                   route(exit_code)
      wg.Wait()  # drains all goroutines before returning
  if daemon idle after dispatch:
    check_fitness_if_idle()
    analyze_run_ledger()
  sleep(poll_interval)
```

**Backoff on errors:** `poll_interval * 2^n`, capped at 5 minutes. Resets after a successful cycle.

### Exit Code Routing

| Exit | Meaning | Action |
|------|---------|--------|
| `0` | More phases remain | Poll again immediately |
| `2` | Human needed | Skip quest until manual intervention |
| `42` | Quest complete | Log completion, move on |
| `1` | Error | Log, retry once, then skip |
| Other | Unexpected | Treat as error |

---

## Subprocess Model

Three dispatch methods, selected per quest type and phase:

### SpawnZeusStep (default)

```
ol zeus step --quest <id>
```

Zeus internally decides whether to call `claude -p` or run validation commands. The daemon never knows or cares. Used for solo phases (most of the Zeus phase machine).

### SpawnTeamCoordinator (team phases)

```
claude -p "/rpi --from=crank <quest>" --allowedTools ...
```

Used when `zeus.PhaseTeam(phase).NeedsTeam()` returns true. Runs from CRANK phase end-to-end with a team of agents. Environment cleaned via `cleanEnvForClaude()` to prevent nesting detection.

### SpawnBidCoordinator (bid quests)

```
claude -p "/implement <bead>"     # if goal is an existing bead ID
claude -p "/rpi \"<goal>\""       # if goal is free text
```

Used exclusively for bid-dispatched quests. Exit code 0 = quest complete (bid coordinator runs monolithically, not phased).

### Common Behavior

**Tracking:**
- Record child PID in `~/.olympus/state/events.jsonl`
- Capture stdout/stderr to `~/.olympus/state/logs/<quest-id>/`
- Log naming: `<timestamp>-{stdout,stderr}.log`, `<timestamp>-team-*.log`, `<timestamp>-bid-*.log`
- Enforce timeout: SIGTERM, wait 30s, SIGKILL
- Set `GIT_AUTHOR_NAME`/`GIT_AUTHOR_EMAIL` for commit attribution

**Timeout:** 15 minutes default, configurable per `daemon.subprocess_timeout`. One timeout fits all phases in Phase 0.

---

## Bid System

Bids are fire-and-forget work requests. Users submit goals; the daemon converts them to quests and executes them autonomously.

### Bid Lifecycle

```
User: ol bid "fix the login bug"
  → BidItem written to ~/.olympus/state/bids/<id>.goal
  → Daemon discovers bid on next pollCycle (before quest discovery)
  → If goal matches bead ID pattern: activate existing bead
  → If free text: bd create "[bid] <goal>" → new quest
  → Write bid marker: quest-<id>.bid (survives restart)
  → Archive bid to ~/.olympus/state/bids/processed/
  → Dispatch via SpawnBidCoordinator
  → On completion: remove bid marker, clean up tracking
```

### BidItem

```go
type BidItem struct {
    ID        string    // bid-<unix_timestamp>-<6hex_random>
    Goal      string    // work description or bead ID
    BeadID    string    // set if goal matches bead ID pattern
    Requester string    // who requested it ($USER default)
    Priority  int       // 0-4 (lower = higher urgency)
    Mode      string    // "auto-rpi"
    CreatedAt time.Time
}
```

**Bead ID detection:** Pattern `^[a-z]{2,4}-[a-z0-9]{2,6}$` (e.g., `ol-jgmk`, `ag-5k2`). If the goal matches, the daemon activates the existing bead instead of creating a new quest.

**Sorting:** Priority ascending, then CreatedAt FIFO.

**Concurrency:** Bids respect `max_concurrent` capacity. If at capacity, bids are queued until the next poll cycle.

### Bid Exit Code Routing

| Exit | Action |
|------|--------|
| `0` | Quest complete — remove active marker, clean up bid tracking |
| `1` | Error — retry once, then skip |
| Other | Treat as error |

Bid quests do NOT use the standard phased exit codes (0=more phases, 42=done). The bid coordinator runs monolithically; exit 0 means done.

### Marker Persistence

On daemon restart, bid quest state is reconstructed from `quest-*.bid` marker files on disk. The in-memory `bidQuests` map is rebuilt, ensuring bid-specific exit code routing survives crashes.

---

## Fitness Checking (Autonomous Health Monitoring)

When the daemon is idle (no active quests) and `fitness_interval` has elapsed, it measures workspace health and creates maintenance quests for regressions.

### Flow

1. Measure current fitness via `ao measure` (if ao is installed)
2. Load baseline from `~/.olympus/state/fitness-baseline.json`
3. Compare: `fitness.CompareFitness(baseline, current)` → list of regressed goals
4. If regressed goals >= `fitness_threshold`:
   - Check rate limit (only one auto-created quest active at a time)
   - Create maintenance quest: `bd create "maintenance: N fitness goals regressed"`
   - Activate quest for daemon pickup
   - Log `maintenance_quest_created` event

### Baseline Capture

- On daemon start (non-fatal if ao is missing)
- After quest completion (exit 0 or 42)
- Saved to `~/.olympus/state/fitness-baseline.json`

### Rate Limiting

Only one auto-created quest (maintenance or improvement) active at a time. Subsequent regressions are skipped until the active quest completes.

---

## Run Ledger Analysis (Pattern Detection)

When the daemon is idle and `analysis_interval` has elapsed, it analyzes recent events to detect failure patterns and creates improvement quests.

### Flow

1. Read last `analysis_window` events from `events.jsonl`
2. Run `AnalyzeEvents(events, config)` with thresholds (failure rate, escalation rate, error rate, stagnation)
3. For each detected pattern, log `pattern_detected` event
4. If patterns found and no auto-created quest active:
   - Select highest-severity pattern
   - Create improvement quest: `bd create "improve: <pattern description>"`
   - Activate quest
   - Log `improvement_quest_created` event

### Severity

`high` (3) > `medium` (2) > `low` (1). Highest severity pattern wins when multiple are detected.

---

## CLI Surface

### `ol daemon start`

```bash
ol daemon start                            # Start daemon
ol daemon start --poll-interval 60s        # Custom poll interval
ol daemon start --fitness-interval 5m      # Custom fitness check interval
ol daemon start --max-concurrent 4         # Enable concurrent quest execution
```

Startup:
1. Validate `~/.olympus/config.yaml`
2. Check PID lockfile — refuse if another daemon is alive (stale locks auto-cleaned)
3. Check for Gas Town daemon — refuse if running
4. Reconstruct bid quest state from `quest-*.bid` markers
5. Write PID to `~/.olympus/state/daemon.pid`
6. Capture fitness baseline (non-fatal if ao missing)
7. Enter poll loop

### `ol daemon stop`

```bash
ol daemon stop               # SIGTERM → graceful
```

On SIGTERM: finish current sleep/wait, SIGTERM child if running, wait 30s, SIGKILL if needed, remove PID file, exit.

### `ol daemon status`

```
$ ol daemon status
Daemon: running (PID 12345)
State: dispatching
Daemon ID: daemon:12345
Last Poll: 2026-02-21T00:05:22Z
Uptime: 2h15m10s
```

### `ol daemon logs`

```bash
ol daemon logs               # Last 20 events
ol daemon logs --follow      # Tail
ol daemon logs --quest <id>  # Filter
```

---

## Crash Recovery

launchd keeps the daemon alive (`KeepAlive: true`, `ThrottleInterval: 30`).

On restart after crash:

1. **Stale lock:** PID file references dead process — delete it, proceed
2. **Orphan subprocess:** Last `subprocess_spawned` event has no matching `subprocess_completed` — SIGTERM the orphan PID if still alive
3. **Bid quest reconstruction:** Scan for `quest-*.bid` files, rebuild in-memory `bidQuests` map so bid-specific exit code routing is restored
4. **HERO orphan recovery:** For each active quest, call `RecoverOrphanedBeads()` — releases beads that were embarked but never ratcheted (actual bd status update at next HERO cycle)
5. **Quest state:** Read bead state + quest checkpoint from disk — resume from last completed phase. Phases are idempotent; re-running is safe.

The filesystem is the source of truth.

---

## Gas Town Migration

### `ol init`

Imports the rig registry:

```bash
ol init                      # Reads ~/gt/mayor/rigs.json → ~/.olympus/config.yaml
```

Converts each rig entry (name, git URL, beads prefix, path) into the `rigs[]` array in config.

### Dual-Daemon Guard

`ol daemon start` checks for a Gas Town daemon process. If detected:

```
Error: Gas Town daemon detected (PID 12345). Run `gt daemon stop` first.
```

One workspace daemon at a time. Both use beads as state, so quest data is compatible — you can switch back and forth.

---

## Key Constraints

| Constraint | Value | Rationale |
|------------|-------|-----------|
| Min poll interval | 30s | Don't hammer the filesystem |
| Default poll interval | 60s | Balance responsiveness vs. overhead |
| Subprocess timeout | 15min | Kill stuck sessions, don't block the loop |
| Max concurrent quests | configurable (default 1) | Backward compat; set max_concurrent in config.yaml or --max-concurrent flag |
| PID lockfile | `~/.olympus/state/daemon.pid` | Single-instance guard |
| Communication model | Filesystem only | No sockets, no IPC, no Agent Mail |
| LLM calls in daemon | 0 | Daemon is mechanical; intelligence is in subprocesses |

---

## Event Log

`~/.olympus/state/events.jsonl` — append-only, one JSON object per line.

Event schema is versioned (`version=2`). `kind` remains for backward compatibility;
new consumers should prefer `event_type` and `reason_code`.

Minimum v2 fields:

- `version`
- `event_type`
- `daemon_id`
- `reason_code`
- `timestamp`

Subprocess lifecycle events additionally carry:

- `quest_id`
- `dispatch_mode`
- `attempt`
- `result`
- `exit_code` (for completion events)

| Event Kind | When |
|------------|------|
| `started` / `stopped` | Daemon lifecycle |
| `poll` | Each poll cycle begin |
| `bid_accepted` | Bid converted to quest |
| `subprocess_spawned` / `subprocess_completed` | Process lifecycle (includes PID, exit code, duration) |
| `fitness_check` / `fitness_regression` | Fitness monitoring |
| `maintenance_quest_created` | Auto-quest for regressions |
| `analysis_triggered` / `pattern_detected` | Run ledger analysis |
| `improvement_quest_created` | Auto-quest for patterns |
| `hero_orphan_recovery` | Orphaned beads recovered |
| `team_heartbeat_killed` | Stale team coordinator killed |
| `draining` | Graceful shutdown in progress |
| `error` | Generic error |

---

## Disk Layout

```
~/.olympus/
├── config.yaml                        # Rigs, daemon settings
├── state/
│   ├── daemon.pid                     # PID lockfile
│   ├── daemon-status.json             # Canonical daemon snapshot (CLI/HTTP/MCP)
│   ├── events.jsonl                   # Append-only event log
│   ├── fitness-baseline.json          # Last known fitness state
│   ├── bids/
│   │   ├── bid-<ts>-<hex>.goal        # Pending bids (JSON)
│   │   └── processed/
│   │       └── bid-<ts>-<hex>.goal    # Archived bids
│   └── logs/<quest-id>/
│       ├── <ts>-stdout.log
│       ├── <ts>-stderr.log
│       ├── <ts>-team-stdout.log       # Team coordinator logs
│       └── <ts>-bid-stdout.log        # Bid coordinator logs
├── zeus/<quest-id>/
│   └── checkpoint.yaml                # Current phase + state
└── launchd/
    └── com.olympus.daemon.plist
```

Quest bid markers: `~/.olympus/state/quest-<id>.bid` (contains goal text, survives restart).

launchd plist uses `KeepAlive: true` and `ThrottleInterval: 30`. It sources `~/.olympus/env.sh` (PATH + API keys) since launchd does not load shell profiles. `ol daemon start --launchd` generates and loads the plist.
