# Vibing With NTM Playbook

<!-- TOC: Spawn and Monitor | Operator Tending | Queue-Dry Protocol | Reservations & Isolation | Monitoring & Output | Swarm Anti-Patterns | Agent Lifecycle | Troubleshooting | FAQ | Validation | Robot Health Cheatsheet | Orchestrator Cadence | Convergence Termination | Domain Assignment | Scope Discipline | Agent-Type Pool Awareness -->

Use this file when the main skill body is not enough and you want the denser
operational playbook.

## Spawn and Monitor

```bash
ntm config get projects_base
ntm quick myproject --template=go

ntm spawn myproject --cc=3 --cod=2 --gmi=1
ntm spawn myproject --cc=5 --cod=5 --no-user
ntm spawn myproject --label frontend --cc=3
ntm spawn myproject --label backend --cc=2 --worktrees
ntm spawn myproject --stagger-mode=smart --cc=6 --cod=4
ntm add myproject --cc=2

ntm list
ntm status myproject
ntm attach myproject
ntm dashboard myproject
ntm view myproject
```

### Recommended Starting Sizes

- `--cc=3 --cod=2 --gmi=1`: strong default
- `--cc=5`: deep reasoning, lower coordination overhead
- `--cc=5 --cod=5`: only when the operator loop is already working well

## Operator Tending

```bash
ntm --robot-snapshot
ntm --robot-attention --attention-cursor=42
ntm --robot-markdown --md-compact
ntm --robot-terse
ntm --robot-mail-check --mail-project=myproject --urgent-only

ntm mail inbox myproject
ntm locks list myproject --all-agents
ntm coordinator digest myproject
ntm coordinator conflicts myproject
ntm work triage --format=markdown
ntm assign myproject --auto --strategy=dependency
```

### Attention Profiles and Waits

Useful attention profiles:

- `operator`
- `debug`
- `minimal`
- `alerts`

Useful wait conditions:

- `idle`
- `action_required`
- `mail_pending`
- `reservation_conflict`
- `file_conflict`

If the cursor expires, re-run `ntm --robot-snapshot`.

## Queue-Dry Protocol

Use this when panes need work but the ready queue appears empty. The goal is to
distinguish three states: ready work exists, the queue is genuinely dry, or the
work graph is degraded/stale.

```bash
br ready --json
bv --robot-triage | jq '.quick_ref'
ntm work queue-dry --format=json | jq '{queue_dry, evidence, recommendations, warnings}'
```

Branching:

- `queue_dry=false`: assign ready work; do not ideate.
- `queue_dry=true` with clean warnings: stand down or preview future work.
- degraded warnings: keep the flow non-mutating and fix/check the native source.
- approved creation: `ntm work queue-dry --ideate --create-beads --yes --plan-version=<token>`, then `br dep cycles --json`, `bv --robot-triage`, and `br sync --flush-only`.

## Reservations and Isolation

Default coordination model:

- Beads decide what should happen.
- Agent Mail says who is doing it.
- File reservations or worktrees prevent collisions.
- NTM surfaces shared state and lets the operator intervene.

Agent Mail reservation example:

```python
file_reservation_paths(
    project_key="/path/to/project",
    agent_name="GreenCastle",
    paths=["internal/auth/**/*.go"],
    ttl_seconds=3600,
    exclusive=True,
    reason="br-123"
)
```

If repo policy allows worktrees, they are also valid:

```bash
ntm spawn myproject --cc=3 --worktrees
ntm worktrees list
ntm worktrees merge claude_1
```

## High-Value Monitoring and Output

```bash
ntm activity myproject --watch
ntm health myproject
ntm watch myproject --cc
ntm copy myproject --all
ntm copy myproject --code
ntm grep "panic" myproject -C 3
ntm logs myproject --panes=1,2
```

## Swarm Anti-Patterns

### Communication Purgatory

Agents keep talking about coordination without taking work.

Fix: require one explicit task owner, one scope claim, and immediate execution.

### File Thrashing

Multiple agents edit the same file or same logical area.

Fix: reserve scope first. When appropriate, move heavy-isolation work to worktrees. Pick a canonical owner when conflicts appear.

### DONE-But-Not-Closed Work

Work is functionally done but the bead state is stale, so downstream work stays blocked.

Fix: periodically check the actual graph state with `bv --robot-triage`, `ntm work triage`, and the relevant `br` status commands.

### Broken Build Drift

One agent breaks the baseline and the swarm keeps coding as if nothing happened.

Fix: announce that the build is broken, route one or two agents to repair it, and obey the repo's heavy-verification rules such as offloading via `rch` when required.

### Idle Agent Drift

An agent stops making progress but still exists in the swarm.

Fix: inspect `--robot-snapshot`, `--robot-tail`, current bead ownership, and whether the agent actually needs a new targeted prompt instead of a vague nudge.

## Agent Lifecycle

### Adding Agents Mid-Session

```bash
ntm add myproject --cc=2
ntm send myproject --panes=5,6 "Read AGENTS.md, check mail, and pick ready work."
```

### Replacing a Dead Agent

```bash
br list --status in_progress
ntm add myproject --cc=1
ntm send myproject --cc "$(cat marching_orders.txt)"
```

### Graceful Shutdown

```bash
ntm send myproject --all "Checkpoint current work and report blockers before stopping."
ntm activity myproject --watch
ntm checkpoint save myproject -m "swarm shutdown"
```

## Troubleshooting

| Problem | What to do |
| --- | --- |
| `spawn` asks to create a directory | Resolve the repo through `projects_base`, use `ntm quick`, or make the repo discoverable from the configured base |
| No ready work | Run `bv --robot-triage`, `bv --robot-next`, or `ntm work triage` |
| Coordination chaos | Check Agent Mail inboxes, lock state, and coordinator digest/conflicts |
| Agent compacted | Re-read repo instructions, then re-check bead, inbox, and `--robot-snapshot` |
| Cursor expired | Re-run `ntm --robot-snapshot` |
| Build broken by swarm | Broadcast the failure, stop speculative work, and route a repair slice immediately |
| Agent looks dead | Inspect `ntm --robot-tail`, bead ownership, and whether it actually needs retasking or replacement |

## FAQ

**How do agents know what to work on?**  
Use `bv --robot-triage`, `bv --robot-next`, or `ntm work triage`.

**How do they avoid conflicts?**  
With file reservations by default, and optionally worktrees when repo policy allows them.

**How many agents should I start with?**  
Usually 3-6. Scale up only when the operator loop is already healthy.

**What is the operator's main monitoring loop?**  
`--robot-snapshot` -> `--robot-attention` / `--robot-wait` -> act -> repeat.

## Validation

The swarm is healthy when:

```bash
ntm dashboard myproject
br list --status in_progress
ntm --robot-terse
ntm --robot-snapshot
ntm --robot-is-working=myproject | jq '[.panes[] | select(.is_working)] | length'
git -C /path/to/project log --since="1 hour ago" --oneline | wc -l  # > 0 during active work
```

Repo-specific verification commands still matter. If the repo `AGENTS.md` says to
offload heavy builds or tests through a helper such as `rch`, follow that rule.

## Robot Health Cheatsheet (the signals the orchestrator should actually trust)

Prefer these surfaces over `ntm activity` / `ntm health` — the newer robot surfaces sample live pane buffers and provider state, whereas the legacy surfaces have repeatedly surfaced stale ("56 years ago") timestamps.

```bash
ntm --robot-is-working=SESSION                        # per-pane: is_working, is_idle, is_rate_limited, is_context_low, recommendation
ntm --robot-agent-health=SESSION                      # comprehensive health, includes CAAM quota
ntm --robot-health-oauth=SESSION                      # OAuth + rate-limit status per provider (catches stale "resets 3pm" messages)
ntm --robot-health-restart-stuck=SESSION --stuck-threshold=10m [--dry-run]   # find/restart panes with no output for N min
ntm --robot-diagnose=SESSION --diagnose-fix           # comprehensive check + auto-fix where possible
ntm --robot-smart-restart=SESSION --panes=N --prompt="..." [--hard-kill]   # safely restart after checking activity
ntm --robot-restart-pane=SESSION --panes=N --restart-prompt="..."          # nuclear: tmux respawn-pane -k, no CLI cooperation needed
ntm --robot-wait=SESSION --wait-until=rate_limited --timeout=30m            # sleep until the rate-limit wall drops
ntm --robot-quota-status                              # CAAM quota across all providers
ntm --robot-account-status                            # CAAM account state, including "next-healthy" for rotation
ntm --robot-switch-account=claude:<account>           # explicit global CAAM account switch
ntm rotate SESSION --all-limited                      # rotate all rate-limited panes at once via CAAM restart strategy
```

## Pressure-Aware Assignment

Before pushing work to many panes, ask whether the bottleneck is actually CPU,
build hosts, provider quota, or reservations:

```bash
ntm --robot-agent-health=SESSION
ntm --robot-rch-status
ntm --robot-quota-status
ntm locks list SESSION --all-agents
```

If any pool is hot, narrow the assignment:

```bash
ntm assign SESSION --auto --strategy=dependency --limit=2
ntm --robot-send=SESSION --panes=N --msg="Take one blocker-clearing bead in your domain; no broad build sweeps until pressure drops."
```

Adding more agents is not the fix for a saturated shared bottleneck. Either wait,
route to a different provider pool, or assign only blocker-clearing work.

## Orchestrator Cadence

Pick an interval and stick to it. Don't sub-3-min poll — it burns tokens.

| Situation | Interval |
| --- | --- |
| Just dispatched fresh prompts; watching for nucleation | 4 min |
| Steady state, mix of working and idle panes | 10–17 min |
| Most panes deep in real work; last tick surfaced nothing new | 30 min |

### Close / review prompt rotation

Drive the mix by backlog depth:

| open + claimed + in_progress | Close : Review |
| --- | --- |
| < 50 | 1 : 3 |
| 50–100 | 1 : 1 |
| 100–200 | 3 : 1 |
| > 200 | close-only |

When open count stops trending down, switch to close-only and block new review-bead creation in the dispatch prompt.

## Convergence Termination

Auto-terminate the orchestrator loop when ALL of:

1. `git log --since="1 hour ago"` shows 0 commits attributed to swarm agents.
2. ≥2 consecutive ticks where every pane produced convergence language ("exemplary", "already complete", "no fixes needed", "ready to ship").
3. `br ready --json` returns 0 items AND `br list --status=in_progress,claimed` is empty or unchanged between ticks.

When all three hold: stop, report, exit. Infinite nudging a converged swarm produces diminishing returns and annoys the user.

## Spawn-Time Domain Assignment

For any swarm with ≥3 agents on a multi-crate or multi-directory workspace, assign explicit domains at spawn time. This is the single biggest productivity lever for wide workspaces.

```bash
# 28-crate Rust workspace example
ntm spawn myproject --cc=2 --cod=4 --gmi=1

# Then in the initial marching orders, tell each pane its domain:
ntm --robot-send=myproject --panes=1 --msg="You own fcp-core, fcp-kernel, fcp-policy. Domain rules: don't edit outside this set without reservation + announcement."
ntm --robot-send=myproject --panes=2 --msg="You own fcp-host, fcp-sdk, fcp-manifest, fcp-bootstrap, fcp-sandbox."
ntm --robot-send=myproject --panes=3 --msg="You own fcp-mesh, fcp-store, fcp-raptorq, fcp-tailscale, fcp-registry."
# ... etc

# Also enable coordinator auto-assign so ntm can pick up collisions automatically:
ntm coordinator enable auto-assign
ntm coordinator enable digest --interval=15m
```

## Scope Discipline

Throughput is bounded by operator attention. Evidence-based heuristic:

- **3 projects × 2-3 agents** is the sweet spot for a single orchestrator.
- **7+ projects** saturates the orchestrator; panes sit idle at prompt boxes for hours before being nudged.

When you find yourself running a 7-repo swarm with 15+ panes, scope down to the 3 projects that are actually moving code and pause the rest until they catch up or you add a second orchestrator tier.

## Agent-Type Pool Awareness

- `cc` panes consume **Claude Max** subscription quota — when one hits a rate limit, all cc panes for the same account are limited.
- `cod` panes consume **ChatGPT Pro** quota — a different pool.
- `gmi` panes consume **Gemini Ultra** quota — a third pool.

When cc is rate-limited across the board, `ntm add myproject --cod=2` to keep the swarm productive on a different pool. Conversely, if the session is pure cc, account-rotation via `caam` / `ntm rotate` is the only escape.
