> **Status:** olympus v4 reference, NOT canonical for agentopsd. This file is a verbatim port from olympus/docs/specs/v4/ for cross-reference only. Where this disagrees with agentopsd's canonical design at `.agents/design/2026-04-28-design-agentops-daemon-gascity-vertical-slices.md`, **agentopsd canonical wins**.

# Olympus v4 CLI Specification (`ol`)

**Status:** Normative
**Date:** 2026-02-15
**Version:** 4.0.0

---

## Philosophy

`ol` does four things:

1. **Assembles context** — Build deterministic context bundles for agent work.
2. **Validates work** — Run mechanical gates no LLM can override.
3. **Manages quests** — Create, track, and complete units of coordinated work.
4. **Extracts knowledge** — Harvest learnings and compile them to constraint tests.

Everything else lives in composed CLIs (bd for issues, ao for knowledge lifecycle) or was cut.

---

## Global Flags

```
--config FILE        Config file (default: ~/.olympus/config.yaml)
-o, --output FORMAT  Output format: json, table (default: table)
-v, --verbose        Verbose output
--dry-run            Show what would happen without executing
--version            Print version and exit
```

---

## Command Set (Machine-Readable)

Tests use this block to prevent drift between spec and compiled Cobra tree.

<!-- BEGIN OL CLI COMMANDS -->
ol bid
ol bid cancel
ol bid create
ol bid list
ol bid status
ol init
ol quest create
ol quest list
ol quest show
ol quest complete
ol quest cancel
ol quest prune
ol hero hunt
ol hero embark
ol hero ratchet
ol hero odyssey
ol hero resume
ol zeus step
ol zeus status
ol zeus run
ol zeus advance
ol validate stage1
ol validate stage2
ol validate consensus
ol context build
ol context diff
ol context runs
ol knowledge harvest
ol knowledge constraints
ol knowledge list
ol knowledge orphaned-followups
ol knowledge show
ol daemon start
ol daemon stop
ol daemon status
ol daemon logs
ol daemon metrics
ol daemon audit
ol primitive
ol primitive run
ol primitive list
ol primitive suggest
ol connect
ol feed
ol mcp
ol mcp serve
ol serve
ol version
<!-- END OL CLI COMMANDS -->

Exempt (Cobra auto-generated): `ol help`, `ol completion`.

**46 commands.**

---

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success / more phases remain |
| `1` | Error |
| `2` | Human needed (approval gate, budget exceeded) |
| `42` | Quest complete (all phases done) |

All commands use 0/1 unless documented otherwise.

---

## Command Reference

### `ol bid`

Parent command for bid dispatch. Not invoked directly — use `ol bid create` or pass a goal string directly (`ol bid "goal"`).

### `ol bid create`

Dispatch a goal to the daemon for autonomous RPI execution. The daemon discovers bids on its next poll cycle, converts them to quests, and dispatches via a bid coordinator.

If the goal matches a bead ID pattern (`^[a-z]{2,4}-[a-z0-9]{2,6}$`), the daemon activates the existing bead instead of creating a new quest.

```
ol bid create <goal> [--priority N] [--requester NAME]
ol bid <goal>                   # shorthand for ol bid create

  --priority N      Priority 0-4 (0=critical, 2=normal, 4=backlog; default: 2)
  --requester NAME  Requester name (default: $USER)
```

### `ol bid cancel`

Cancel a pending bid before the daemon picks it up. Already-processed bids cannot be cancelled.

```
ol bid cancel <bid-id>
```

### `ol bid list`

Show pending and processed bids in the daemon queue.

```
ol bid list
```

### `ol bid status`

Show detailed status of a specific bid.

```
ol bid status <bid-id>
```

### `ol init`

Bootstrap `~/.olympus/` with default config. Safe to re-run; merges without overwriting.

```
ol init [--force]

  --force    Overwrite existing config

Creates:
  ~/.olympus/config.yaml    Daemon + rig settings
  ~/.olympus/state/         Daemon event log directory
```

### `ol quest create`

Create a quest. Makes an epic bead via bd and a `quest/<id>` branch from main.

```
ol quest create <title> [--spec PATH] [--priority HIGH|NORMAL|LOW]
```

### `ol quest list`

List quests with aggregated bead status.

```
ol quest list [--status planning|running|complete|failed]
```

### `ol quest show`

Show quest details and child bead progress.

```
ol quest show <quest-id> [-v]
```

### `ol quest complete`

Complete a quest. All child beads must be closed. Creates a PR from `quest/<id>` to main.

```
ol quest complete <quest-id>

Exit codes: 0=done, 1=error, 2=beads still open
```

### `ol quest cancel`

Cancel a quest and close its epic bead.

```
ol quest cancel <quest-id> --reason <text>
```

### `ol quest prune`

Delete local quest branches that are fully merged into main. Safe: uses `git branch -d`.

```
ol quest prune [--dry-run]
```

### `ol hero hunt`

Find ready beads for a quest. Returns the next wave as JSON: `{wave, blocked, completed}`.

```
ol hero hunt --quest <id> [--capacity N]

  --capacity N    Max beads in wave (default: config max_demigods)
```

### `ol hero embark`

Claim a bead and build its context bundle. Writes prompt and manifest to `.ol/prompts/`.

```
ol hero embark <bead-id> --quest <id> [--attempt N] [--feedback TEXT]

  --attempt N       Attempt number (default: 1)
  --feedback TEXT   Prior attempt failure details
```

### `ol hero ratchet`

Check the run ledger and report pass/fail status for each bead in the quest.

```
ol hero ratchet --quest <id>
```

### `ol hero odyssey`

Handle a failed bead. If retries remain (max 3), rebuilds context with feedback. Otherwise escalates (exit 2).

```
ol hero odyssey <bead-id> --quest <id> [--feedback TEXT]

  --action ACTION    Optional override: retry or escalate (default: auto)

Exit codes: 0=retrying, 1=error, 2=budget exhausted
```

### `ol hero resume`

Detect in-flight HERO state (beads embarked but never ratcheted) and release them so the next HERO cycle can re-claim them. Used for crash recovery — does not re-run embark, only resets stuck beads.

```
ol hero resume --quest <id> [--threshold DURATION]

  --threshold DURATION    Staleness threshold before releasing (default: 2h)
```

### `ol zeus step`

Execute one phase of the Zeus state machine. Call repeatedly from an external loop.

```
ol zeus step --quest <id>

Exit codes: 0=more phases, 1=error, 2=needs work, 42=done

Phase machine:
  RESEARCH -> PLAN -> PLAN_GATE -> PRE_MORTEM -> PRE_MORTEM_GATE ->
  CRANK_PREP -> CRANK_VALIDATE -> VIBE_GATE -> POST_MORTEM ->
  RETRO -> FLYWHEEL -> FITNESS_GATE -> DONE
```

### `ol zeus status`

Show current phase, budget remaining, and last event for a quest.

```
ol zeus status --quest <id>
```

### `ol zeus run`

Run a quest end-to-end autonomously. Dispatches Claude subprocesses for phases that exit 2.

```
ol zeus run --quest <id> [--from PHASE] [--interactive] [--fast] [--stream]
ol zeus run "<goal>"

  --from PHASE      Resume from a specific phase
  --interactive     Pause after RESEARCH and PLAN (exit 2, advance to continue)
  --fast            Skip optional phases (PRE_MORTEM, PRE_MORTEM_GATE, POST_MORTEM, RETRO, FLYWHEEL)
  --stream          Pipe subprocess stdout to terminal in real time

If --quest is omitted, creates a new quest from "<goal>".

Exit codes: 0=complete, 1=error, 2=budget exceeded
Budget: 50 iterations max.
```

### `ol validate`

Parent command for validation gates. Not invoked directly — use `ol validate stage1` or `ol validate stage2`.

### `ol validate stage1`

Run mechanical checks (go build, go vet, go test) in a worktree. Hard gate.

```
ol validate stage1 --quest <id> --bead <id> --worktree <path> [--timeout DURATION]

  --timeout DURATION    Timeout per step (default: 5m)

Results written to .ol/runs/<bead-id>/
```

### `ol validate stage2`

Merge gate. Requires Stage 1 PASS, then merges demigod branch into quest branch.

```
ol validate stage2 --quest <id> --bead <id> --demigod-branch <branch>
```

### `ol context build`

Assemble a deterministic context bundle for a bead.

```
ol context build --bead <id> [--quest <id>] [--attempt N] [--feedback TEXT]

Output:
  .ol/prompts/<bead>-attempt-<n>.md              Rendered prompt
  .ol/prompts/<bead>-attempt-<n>.bundle.json     Bundle manifest
```

### `ol context diff`

Compare context bundles between attempts for the same bead.

```
ol context diff --bead <id> [--from N] [--to N]
```

### `ol context runs`

Show run history for a bead: attempts, validation results, bundle hashes.

```
ol context runs --bead <id> [--attempt N]
```

### `ol knowledge harvest`

Extract learnings from a completed quest's artifacts (git log, bead notes, quest events).

```
ol knowledge harvest <quest-id>
```

### `ol knowledge list`

List harvested learnings with provenance metadata.

```
ol knowledge list [--dir PATH]

  --dir PATH    Learnings directory (default: .agents/learnings/)
```

### `ol knowledge show`

Show a single learning artifact (full markdown + provenance metadata).

```
ol knowledge show <slug> [--dir PATH]

  --dir PATH    Learnings directory (default: .agents/learnings/)
```

### `ol knowledge constraints`

Extract structured constraints from learnings and optionally compile them to Go test files.

```
ol knowledge constraints [--dir PATH] [--generate-tests] [--output-dir PATH]

  --dir PATH           Learnings directory (default: .agents/learnings/)
  --generate-tests     Also emit Go test files
  --output-dir PATH    Test output directory (default: internal/constraints/)
```

### `ol knowledge orphaned-followups`

Scan council reports for FOLLOW-UP markers that were never converted to beads or tracked. Reports count and locations of unextracted follow-up work items.

```
ol knowledge orphaned-followups [--dir PATH]

  --dir PATH    Council directory to scan (default: .agents/council/)
```

### `ol daemon start`

Start the background poll loop. The daemon is mechanical: it polls for ready quests, processes bids, and invokes subprocesses. No LLM runs inside the daemon.

```
ol daemon start [--poll-interval DURATION] [--fitness-interval DURATION] [--max-concurrent N]

  --poll-interval DURATION       Override config poll interval (min: 30s)
  --fitness-interval DURATION    Override config fitness check interval (min: 60s)
  --max-concurrent N             Override config max concurrent quests (default: 1)

PID written to ~/.olympus/state/daemon.pid
```

### `ol daemon stop`

Stop the daemon gracefully.

```
ol daemon stop
```

### `ol daemon status`

Show daemon state snapshot: running/stopped, PID, daemon state, daemon ID, last poll, and uptime.

```
ol daemon status
```

### `ol daemon logs`

Tail the daemon event log. Subprocess events include normalized routing metadata
(`mode`, `attempt`, `result`, `reason`) when available.

```
ol daemon logs [--follow] [--lines N]

  --follow     Stream new events
  --lines N    Show last N lines (default: 50)
```

### `ol daemon metrics`

Aggregate daemon event log into performance metrics. Shows total quests processed, phases completed, average phase duration, error rate, and per-quest breakdown.

```
ol daemon metrics
```

### `ol daemon audit`

Audit daemon state: PID lock status, active quests, subprocess status, orphaned beads, and recent events. Combines daemon status, event log, and orphan detection into a single diagnostic view.

```
ol daemon audit [--quest ID] [--events N] [--cleanup] [--prune-worktrees]

  --quest ID     Quest ID to scope orphan detection
  --events N     Number of recent events to show (default: 20)
  --cleanup      Cleanup stale daemon/zeus state for terminal quests
  --prune-worktrees
                 Run `git worktree prune` as part of --cleanup (default: true)
```

### `ol connect`

Connect to a quest's live event stream. Tails the feed filtered to a specific quest,
showing recent history and then following new events in real time.

```
ol connect --quest <id> [--json] [--lines N]

  --quest ID     Quest ID to follow (required)
  --json         Output raw JSONL without formatting
  --lines N      Number of recent lines to show before following (default: 20)
```

Equivalent to `ol feed --follow --quest <id>` but with required quest ID and
output formatted to highlight phase transitions.

### `ol feed`

Show unified activity stream. Reads quest events from the feed directory
(`OL_FEED_DIR` env var or `feed_dir` config) as a JSONL timeline.

```
ol feed [--follow] [--lines N] [--quest ID] [--json]

  --follow       Stream new events (tail -f behavior)
  --lines N      Show last N lines (default: 50)
  --quest ID     Filter events for a specific quest
  --json         Output raw JSONL without formatting
```

The FeedEmitter writes events to `<feed-dir>/olympus.jsonl` when `OL_FEED_DIR`
is set. Exit code 0 always (missing feed data is informational, not an error).

### `ol serve`

Start the Olympus HTTP dashboard and API server for browser-based monitoring.

```
ol serve [--host HOST] [--port PORT] [--open] [--no-auth]

  --host         Host/interface to bind (default: localhost)
  --port         HTTP port to bind (default: 8080)
  --open         Open browser after startup
  --no-auth      Disable Bearer token auth (local dev only)
```

Provides REST API endpoints for quest/bead/daemon status, SSE event streaming,
and an embedded htmx dashboard. Exit code 0 on clean shutdown, 1 on error.

### `ol mcp`

Parent command for MCP server. Not invoked directly — use `ol mcp serve`.

### `ol mcp serve`

Start the Olympus MCP server on stdio transport for IDE integration.

```
ol mcp serve
```

Exposes quest, hero, zeus, context, and validate tools as MCP tools.
Configure in your IDE:

```json
{
  "mcpServers": {
    "olympus": {
      "command": "ol",
      "args": ["mcp", "serve"]
    }
  }
}
```

### `ol primitive`

Parent command for composable primitives. Not invoked directly — use subcommands.

### `ol primitive list`

List all 8 registered primitives with wiring status, or show built-in recipes.

```
ol primitive list [--recipes]

  --recipes    Show built-in recipe sequences instead of primitives
```

### `ol primitive run`

Execute a single primitive by name. Reads input artifacts from the base directory and writes output artifacts.

```
ol primitive run <name> [--quest ID] [--base-dir PATH]

  --quest ID       Quest ID for context
  --base-dir PATH  Base directory for artifacts (default: .)
```

### `ol primitive suggest`

Suggest the next primitives based on execution history. Uses the composer and CanFollow matrix to determine valid next steps.

```
ol primitive suggest [--quest ID]

  --quest ID    Quest ID for context
```

### `ol version`

Print version, commit hash, and build date.

```
ol version
```

---

## What Was Cut (and Why)

| v3 Command | Disposition |
|------------|-------------|
| `ol artifacts` | Use filesystem directly |
| `ol autopilot` | Replaced by daemon + `ol zeus run` |
| `ol config` | Read `~/.olympus/config.yaml` directly |
| `ol dashboard` | Out of scope for v4 |
| `ol debug doctor` | Fold into `ol daemon status` |
| `ol fleet` | Daemon handles concurrency |
| `ol harvest` (top-level) | Moved under `ol knowledge harvest` |
| `ol hero checkpoint/resume/metrics` | Crash recovery from beads state, not checkpoints |
| `ol hydra` | Metrics are internal, not a command |
| `ol knowledge generate-tests/regenerate` | Folded into `ol knowledge constraints --generate-tests` |
| `ol lint` | Runs inside `ol validate stage1` |
| `ol loom` | Out of scope |
| `ol plan approve` | Folded into Zeus gate phases |
| `ol quest create-followup/cycle/events/watch` | Simplify quest surface |
| `ol ready` | Use `bd ready` |
| `ol rollback bead` | Manual git revert; not worth a command |
| `ol rpi` | Skills dispatch via ao, not ol |
| `ol start/stop` (v3 stubs) | Replaced by `ol daemon start/stop` |
| `ol temporal` | Removed; daemon replaces durable execution |
| `ol validate ops` | Folded into Stage 2 when mission_critical is set |
| `ol zeus advance/hotspots/replay/reset` | `step` + `run` cover all cases |
