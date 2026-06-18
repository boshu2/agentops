# Observability & Data Quality Judgment

<!-- TOC: Freshness Thresholds | Degradation Semantics | Attention Lifecycle | Per-Pane Truth | Git Log Ground Truth | Scope & Saturation | Convergence Termination | Three-Observation Rule -->

## Contents

- Source freshness and degradation semantics
- Attention item lifecycle and actionability classes
- Per-pane truth checks and liveness signals that lie
- Convergence and stop rules

The swarm is only as honest as your last probe. Every orchestrator decision should answer two questions:

1. **Is this data fresh enough to act on?**
2. **Am I inferring or observing?**

Inference without observation is how swarms drift for hours while appearing productive.

## Source Freshness Thresholds

From `/dp/ntm/docs/freshness-contract.md`. Each source has its own cadence — apply the right threshold.

| Source | Fresh | Stale (acceptable) | Critical |
| --- | --- | --- | --- |
| `tmux` | <5s | 5–30s | >30s or "no server running" |
| `beads` | <30s | 30–300s | >300s |
| `mail` | <60s | 60–120s | >120s |
| `quota` (caam) | <300s | 300–900s | >900s |
| `rch` | <60s | 60–300s | >300s |
| `coordinator` | <30s | 30–120s | >120s |

Query source health explicitly:

```bash
ntm --robot-snapshot | jq '.sources.sources | to_entries[] | {source: .key, fresh: .value.fresh, degraded: .value.degraded, age_ms: .value.age_ms, reason: (.value.degraded_reason // .value.reason_code)}'
```

### Decision Rule Per Tier

| Tier | Rule | Example |
| --- | --- | --- |
| Fresh | Act normally. Trust the data. | Dispatch new bead assignment. |
| Stale | Act, but annotate the staleness to the agent. | "Beads data is 180s stale; re-check with `br list --json` before editing." |
| Critical | Do not act on this source. Escalate or resync. | Mail down for 10 min → fall back to `br update --assignee` soft lock. |

## Source Degradation Semantics

When a source is stale/unavailable, `--robot-snapshot` returns `degraded_features` on that source. These are features you **cannot trust** while the source is degraded.

| Source | Degraded Features | Implication | Workaround |
| --- | --- | --- | --- |
| `beads` | `ready_count`, `in_progress_list` | Work queue unreliable | Agents re-run `bv --robot-triage` locally; use `br list --json` directly |
| `mail` | `unread_count`, `urgent_threads` | Mail visibility incomplete | Poll inbox with `ntm --robot-mail-check`; fall back to bead-assignee lock |
| `quota` | `remaining_percent`, `resets_at` | Rate-limit truth unclear | Probe pane directly via ping; use `--robot-health-oauth` where possible |
| `tmux` | `agent_states`, `session_list` | Agent list stale | Sample live pane state via `--robot-is-working` |
| `rch` | `worker_health`, `build_queue` | Remote builds unreliable | Route to local `cargo/go/bun` until recovered |

## Attention Item Lifecycle

From `/dp/ntm/docs/robot-attention-state.md`. Every attention item has a state and transitions.

```
  new ─────▶ seen ─────▶ acknowledged ─────▶ (resolved)
    │          │
    │          └──▶ snoozed (timed) ─────▶ seen (when wakes)
    │
    └──▶ dismissed (operator explicit) ─────▶ (gone, will not re-surface unless underlying event re-fires)
```

States, triggers, and operator actions:

| State | Trigger | When You Should | When You Should Not |
| --- | --- | --- | --- |
| `new` | Event just fired | Read it within one tick | Ignore for >1 tick in a row |
| `seen` | Operator observed it | Act or snooze with reason | Leave indefinitely without a decision |
| `acknowledged` | Operator accepted responsibility | Follow up at next tick | Acknowledge without a plan |
| `snoozed` | Deferred with timer | Revisit when it re-surfaces | Snooze repeatedly (= dismissed-but-not-honest) |
| `dismissed` | Operator explicitly chose to ignore | Only when the underlying signal is known-false | Dismiss to silence noise — fix the noise source instead |
| `pinned` | Must stay visible | Use for long-running concerns | Pin every item (clutter) |

### Actionability Classes

Every attention item comes classified:

| Class | Meaning | Operator Response |
| --- | --- | --- |
| `action_required` | Immediate decision needed (rate limit, stuck pane, build break) | Act now. Do not advance the loop without resolving or explicitly deferring. |
| `interesting` | Worth noticing (file conflict warning, low context) | Inspect; may not need immediate action. |
| `background` | Informational (scheduled task fired, idle pane) | Monitor; no action required. |

**Rule:** Never act on `action_required` items based on stale data. Always check `sources` / `degraded_sources` for the source that generated the item first.

## Per-Pane Truth Checks

When the snapshot says "idle" but you suspect the pane is actually working (or vice versa), verify directly.

```bash
# 1. Is it really working?
ntm --robot-is-working=myproject --panes=5 --verbose
# Returns: is_working, is_idle, is_rate_limited, is_context_low, confidence, indicators

# 2. Is the agent alive (different from "working")?
ntm --robot-agent-health=myproject --panes=5 --verbose

# 3. What's actually in the pane right now?
ntm --robot-tail=myproject --lines=30 --panes=5

# 4. What does the OS say?
ps -fp $(tmux display -p -t myproject:0.5 '#{pane_pid}')
```

**Confidence scoring** (from `--robot-is-working`):

| Confidence | Meaning | Action |
| --- | --- | --- |
| >0.8 | High confidence in verdict | Trust and act |
| 0.5–0.8 | Indicators mixed | Cross-check with `--robot-agent-health` or tail |
| <0.5 | Very unsure | Do NOT restart; observe another tick first |

## Git Log as Ground Truth

The only reliable productivity signal is commits landing.

```bash
# Last hour of commits attributed to swarm agents (configure authors via .git/config or CODEOWNERS)
git -C /path/to/repo log --since="1 hour ago" --oneline --format='%ar %an %h %s' | head -30

# Just the count
git -C /path/to/repo log --since="1 hour ago" --oneline | wc -l

# Per-author
git -C /path/to/repo log --since="1 hour ago" --format='%an' | sort | uniq -c | sort -rn
```

**Productivity triangulation** (when deciding if a swarm is genuinely working):

1. `--robot-is-working` says ≥N panes busy → observation
2. `ps -eo comm | grep -cE '^(cargo|rustc|go|bun)$'` ≥1 → build processes running
3. `git log --since="30m ago"` shows ≥1 new commit → output landed

If all three hold: genuinely productive. If only 1 or 2, you might be watching prose generation.

## Scope & Saturation Limits

### Orchestrator attention budget

Evidence-based heuristic from real-world swarm operation:

| Operator tier | Max simultaneous projects | Max panes per project |
| --- | --- | --- |
| Single human | 1 project | 6 panes |
| Single orchestrator agent | 3 projects | 6 panes each (≤18 total) |
| 2-tier (meta-orchestrator + per-project) | 7+ projects | 6 panes each |

Beyond these tiers, panes sit idle at prompt boxes for hours before being nudged — the orchestrator simply cannot keep up.

### Context saturation watermark

Per-pane context use:

| Pane context % used | Interpretation | Action |
| --- | --- | --- |
| <60% | Healthy | Keep working |
| 60–85% | Getting tight | Keep working but avoid new large file reads |
| 85–92% | Pre-compact zone | Send "checkpoint + handoff" prompt |
| >92% | Imminent auto-compact | Preemptive restart (beats waiting for compaction) |

## Deterministic Convergence-Termination

The loop MUST terminate when:

1. `git log --since="1 hour ago"` = 0 commits
2. ≥2 consecutive ticks with every pane producing convergence language ("exemplary", "already complete", "no fixes needed", "ready to ship")
3. `br ready --json` = 0 items AND `br list --status=in_progress,claimed` unchanged between ticks

When all three hold: stop, report, exit. Infinite nudging a converged swarm is the most common way orchestrators waste tokens.

```bash
# Convergence detection primitive
commits_1h=$(git -C "$REPO" log --since="1 hour ago" --oneline | wc -l)
ready=$(br ready --json | jq '.issues | length')
inflight=$(br list --status=in_progress,claimed --json | jq '.issues | length')

if [ "$commits_1h" -eq 0 ] && [ "$ready" -eq 0 ] && [ "$inflight" -eq 0 ]; then
  echo "CONVERGED — terminate loop"
fi
```

## The Three-Observation Rule

Before any state-changing action (restart, rotate, reassign, escalate), have **three independent observations**:

1. Real-time signal (`--robot-is-working`, `--robot-health-oauth`)
2. Persistent signal (git log, bead state, coordinator conflicts)
3. Underlying system signal (ps, df, OAuth endpoint, tmux socket)

If any two disagree, resync before acting. If three agree, act with confidence. One observation is a guess, not a judgment.

## Liveness Signals That Can Lie

Every signal an orchestrator reads comes through some layer that can stale, cache, or misinterpret. Catalog the ones that have burned operators in real sessions so you don't re-discover them.

### Activity-indicator timers are not activity

CLI timers such as "Cogitated for 35m", "Worked for 1h 39m", "Sautéed for 4m", "Fluttering 4m 47s" persist across ticks even while the agent is landing commits — and, conversely, sometimes hold identical values while the pane is genuinely mid-compile. They're display artifacts, not activity counters.

Authoritative cross-references:

```bash
# Real productivity of this pane over the last 15 min:
git -C <repo> log --since='15 minutes ago' --oneline | wc -l

# Real build work happening anywhere on the box:
pgrep -af 'cargo (test|check|bench|build)' | wc -l
ps -eo comm | grep -cE '^(cargo|rustc|go|bun|node)$'
```

If the pane timer is flat AND `git log` shows zero commits AND no build processes match that pane's target dir → genuinely idle. Any one of those three disagreeing → keep observing.

### Rate-limit / paste-buffer text in the buffer is stale by definition

Extends AP-7. The text is rendered once and left in the alt-screen; the CLI doesn't re-check unless the user interacts. Treat any pane-buffer status line as a claim about the past, not the present.

### Agent CLI can silently exit back to zsh

`--robot-tail` will happily return the pane's last N lines of output regardless of whether the agent CLI is still running. A codex/cc pane that crashed or double-C-c'd is now a bare zsh — the next prompt you dispatch lands at the shell as literal text (e.g. `zsh: no matches found: (259f4826)`).

Authoritative signal:

```bash
tmux list-panes -t <session>:<win> -F '#{pane_index} #{pane_current_command} #{pane_pid}'
# pane_current_command of "zsh" (or "bash") means the agent CLI is dead.
# Expected: "claude", "bun" (codex), "gemini", "node" depending on agent type.

# Also track pane_pid — if it matches the pid from your last check, "restart" didn't take.
```

Add this audit to the stuck-pane ladder **before** any interrupt or dispatch.

### "Executing now" indicators that ARE authoritative

Unlike buffer text, two small indicators on the agent CLI track actual in-flight execution:

- `⏵⏵ bypass` indicator at the bottom of a cc pane → actively executing tool calls (rate-limit / resets text higher in scrollback is leftover frames).
- `• Working …` / `• Waiting for background terminal` prefix lines on codex → also authoritative.
- Codex idle-state placeholder suggestions such as `Summarize recent commits` / `Explain this codebase` are NOT a "stuck, waiting for prompt" signal — they're hints shown when no prompt is in flight. Easy to misread and over-nudge.

When these indicators are present, the pane is alive even if timers are frozen. Cross-check against git log and pgrep before redirecting — mid-think panes landing real work should not be interrupted (see AP-47).

### Tmux window index is not always `0`

`pane-base-index` and `base-index` can be set to `1` in `~/.tmux.conf`. Addressing `session:0.<pane>` silently fails with "can't find window: 0" and your keypress disappears. Always discover:

```bash
tmux list-windows -t <session> -F '#{window_index} #{window_name}'
# Use the leading index returned here, not a hardcoded 0.

# Or robust one-liner:
WIN=$(tmux list-windows -t <session> -F '#{window_index}' | head -1)
tmux send-keys -t <session>:${WIN}.<pane> "…" Enter
```

### `--robot-tail` as snapshot, `tmux capture-pane` as ground truth

`--robot-tail` is efficient for bulk surveys across many panes but can sample stale buffer content for several ticks in a row on transient states (keypress echoes, dialog transitions). For verifying "did my last keypress actually land" or "is this dialog gone now", prefer:

```bash
tmux capture-pane -t <session>:<win>.<pane> -p -S -20
# -p = print to stdout, -S -20 = last 20 lines of scrollback
```

Rule: `--robot-tail` for bulk, `capture-pane` for single-pane ground truth after an action.

### `capture-pane` of a codex/full-screen TUI can return ANSI-only / empty

The whole "when the meter lies, read the actual pane via `capture-pane`" fallback (AP-7/AP-41, OC-028, OC-037) assumes capture returns readable text. It does not for every agent. A full-screen TUI — **codex especially** — paints to the alt-screen, so `tmux capture-pane` / `ntm save` returns almost entirely ANSI/alt-screen control sequences; stripping them leaves an empty string. **An empty or ANSI-only capture is NOT evidence that the pane is idle** — the documented fallback itself failed, and concluding "idle" here is the trap.

When capture is unreadable, fall through to the ground-truth rung below; do not restart on a blank capture.

```bash
# capture came back empty/ANSI? get the honest signal from the OS, not the screen:
PID=$(tmux display -p -t <session>:<win>.<pane> '#{pane_pid}')
ps -p "$PID" -o pcpu=,etime=,comm=        # 0.0% + no growing artifact = genuinely idle
# codex-specific structured fallback when available:
ntm --robot-codex-palette-state=<session> --panes=<pane> --format=json 2>/dev/null   # query --robot-capabilities first
```

### Pane-PID CPU% is the signal of last resort

When the status meter lies AND `capture-pane` is unreadable (above), per-pane CPU% is the only honest liveness signal left. It is layer 4 of the SKILL.md Liveness Truth Stack, not a footnote:

```bash
ps -p $(tmux display -p -t <session>:<win>.<pane> '#{pane_pid}') -o pcpu=
# 0.0% on the pane process tree + no growing artifact (no new commit, no build output) = genuinely idle.
# A booted-but-never-engaged pane (dropped first dispatch / boot race) reads exactly this — re-dispatch, don't restart (OC-047).
# CPU above 0 with a live rustc/cargo/bun child = mid-work; leave it (AP-54).
```

Pair it with an artifact check (`git log --since`, build output, mail ack) — CPU% alone distinguishes *idle* from *busy*, the artifact distinguishes *never-engaged* from *standing-down*.

### Disk trajectory beats absolute threshold

A single "disk 73%" reading is a snapshot; the danger signal is **delta-per-tick**. Runaway cargo `target/` dirs can climb `+3pp/tick` for several ticks before cresting, and fuzz corpora / build caches fail silently once the cliff is hit. Track trajectory:

```bash
# Record each tick
df -h / | awk 'NR==2 {print $5}' | tr -d '%' > /tmp/disk_tick.$(date +%s)

# Warn early: threshold 50% + delta > 3pp/tick between the last two samples
```

Signal hierarchy: trajectory > absolute % > targeted sweep. Target per-pane `CARGO_TARGET_DIR=/tmp/build_<proj>_<role>` so sweeps can be scoped per-agent.

### "Degraded" posture ≠ broken

`rch` / coordinator / source-health can report `degraded` when 7/8 workers are healthy and most slots are free. Don't escalate on the warning text alone — look at the underlying counts (`slots_free`, `workers_healthy / workers_total`, `error_rate`). Only escalate on exhaustion or real failures, not on the color of the label.
