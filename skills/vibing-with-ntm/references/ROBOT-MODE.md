# Robot-Mode Surface Catalog

<!-- TOC: Query The Registry | Five Lanes | Top 20 Surfaces | Transport Availability | Deprecated Flags | Known Gotchas | Canonical Operator Loop -->

Every surface here is a machine-readable entry point the orchestrator should prefer over interactive TUIs or raw `tmux`. Always trust `ntm --robot-capabilities` over hand-written docs — the registry is authoritative.

## Always Query The Registry First

```bash
# Discover all surfaces (137 commands in the 2026-05 mainline snapshot; always re-query)
ntm --robot-capabilities | jq '.commands[] | .name'

# Find commands matching a need
ntm --robot-capabilities | jq '.commands[] | select(.name | test("rate|limit|health|restart|wait"))'

# Registry-derived docs and command entries (always current)
ntm --robot-docs=commands
ntm --robot-capabilities | jq '.commands[] | select(.flag=="--robot-wait")'
ntm --robot-capabilities | jq '.commands[] | select(.flag=="--robot-is-working")'
```

If a surface you remember isn't in `--robot-capabilities`, the CLI is a newer build than your memory. Do not invent flags. Query and adapt.

## The Five Lanes (Operator Loop Phases)

| Lane | Phase | Canonical Surfaces | Use When |
| --- | --- | --- | --- |
| `bootstrap` | Establish baseline | `--robot-snapshot`, `--robot-spawn`, `--robot-status` | Starting a session; resuming after cursor expiration |
| `summarize` | Quick glance | `--robot-terse`, `--robot-markdown --md-compact`, `--robot-dashboard` | Checking health before deciding next action |
| `replay` | Audit trail | `--robot-events --since-cursor=X`, `--robot-digest` | Understanding what changed since last cursor and summarizing the feed |
| `triage` | Prioritize | `--robot-attention`, `--robot-wait`, `--robot-alerts` | Waiting for blocking items; deciding what to act on |
| `inspect` | Drill down | `--robot-tail`, `--robot-is-working`, `--robot-agent-health`, `--robot-diagnose` | Understanding details after triage |
| `act` | Take action | `--robot-send`, `--robot-interrupt`, `--robot-smart-restart`, `--robot-restart-pane`, `--robot-switch-account` | Dispatching work or recovering stuck agents |
| `wait` | Pause | `--robot-wait --wait-until=...` | Blocking until condition met or resource ready |

## Top 20 Surfaces Every Orchestrator Should Know

### Bootstrap & Session State

```bash
# Full state: sessions + beads + alerts + mail + source health + cursor for follow-ups
ntm --robot-snapshot

# Cheap quick glance (health score + ready work + alerts) — prefer over full snapshot for rapid polls
ntm --robot-terse

# Human-readable dump for operator briefings / session notes
ntm --robot-markdown --md-compact
```

### Event Stream (replaces manual polling)

```bash
# Incremental events since cursor — canonical audit trail
ntm --robot-events --since-cursor=<cursor> --events-actionability=action_required

# Token-efficient non-blocking summary of the attention feed
ntm --robot-digest --profile=minimal

# Blocking wait for next attention-worthy event (top prioritized item)
ntm --robot-attention --attention-timeout=30s --attention-cursor=<cursor>
```

**Cursor semantics:** monotonic, garbage-collected after 1 hour. If `CURSOR_EXPIRED` returns, resync via `--robot-snapshot` and continue.

### Pane Truth (prefer over `tmux capture-pane`)

```bash
# Per-pane work state — authoritative liveness check
ntm --robot-is-working=myproject --panes=2,3,4
#   Returns per pane: is_working, is_idle, is_rate_limited, is_context_low, confidence, recommendation

# Detailed drill-down on one pane
ntm --robot-inspect-pane=myproject --inspect-index=5

# Structured tail (handles alt-screen; better than tmux capture-pane)
ntm --robot-tail=myproject --lines=50 --panes=2,3,4

# Comprehensive per-pane health including provider data
ntm --robot-agent-health=myproject

# Errors only (filters noise from the tail)
ntm --robot-errors=myproject
```

### Provider & Quota Truth (catches stale "resets 3pm" lies)

```bash
# Live OAuth + rate-limit status per pane — truth, not the stale pane buffer message
ntm --robot-health-oauth=myproject

# CAAM quota across all providers — global view
ntm --robot-quota-status

# Per-provider drill-down
ntm --robot-quota-check --provider=claude

# Account rotation (cc and cod use different pools)
ntm --robot-account-status
ntm --robot-accounts-list --provider=claude
ntm --robot-switch-account=claude:jeff2718281
```

### Stuck-Pane Detection & Restart

```bash
# Detect panes with no output for N minutes — dry-run preview first
ntm --robot-health-restart-stuck=myproject --stuck-threshold=10m --dry-run
ntm --robot-health-restart-stuck=myproject --stuck-threshold=10m      # actually restart

# Smart: check activity, avoid trashing real work
ntm --robot-smart-restart=myproject --panes=5 --prompt="$(cat marching_orders.txt)"

# Hard-kill fallback (when graceful shutdown wedges on /usage or a confirm dialog)
ntm --robot-smart-restart=myproject --panes=5 --hard-kill --prompt="..."

# Nuclear: tmux respawn-pane -k, bypasses CLI cooperation entirely
ntm --robot-restart-pane=myproject --panes=5 --restart-prompt="..."
ntm --robot-restart-pane=myproject --panes=5 --restart-bead=br-123     # template from bead
```

### Non-Interactive Dispatch

```bash
# Non-interactive (no confirm dialogs, no CASS duplicate check)
ntm --robot-send=myproject --panes=2,3 --msg="<prompt>"

# Interactive send — WARNING: blocks on CASS duplicate detection dialog in orchestrator loops
ntm send myproject --pane=5 --no-cass-check "<prompt>"           # use --no-cass-check in loops
ntm send myproject --all --skip-first "<prompt>"                 # --skip-first excludes user pane 1

# Interrupt + optional new task
ntm --robot-interrupt=myproject --panes=5 --msg="..."

# Mail check (non-interactive, urgent-only filter)
ntm --robot-mail-check --mail-project=myproject --urgent-only
```

### Waits & Conditions

```bash
# Block until condition met
ntm --robot-wait=myproject --wait-until=idle --timeout=10m
ntm --robot-wait=myproject --wait-until=rate_limited --timeout=30m     # wake when wall drops
ntm --robot-wait=myproject --wait-until=mail_pending --timeout=5m
ntm --robot-wait=myproject --wait-until=reservation_conflict --timeout=2m
ntm --robot-wait=myproject --wait-until=attention --attention-cursor=<cursor> --profile=operator

# Full condition list from --robot-capabilities:
#   idle, complete, generating, healthy, stalled, rate_limited,
#   attention, action_required, mail_pending, mail_ack_required,
#   context_hot, reservation_conflict, file_conflict,
#   session_changed, pane_changed
#   (bead_orphaned deliberately unsupported — polling would race)
```

### Work Assignment

```bash
# Get assignment recommendations (does not execute)
ntm --robot-assign=myproject

# Bulk assign beads across idle agents via bv triage
ntm --robot-bulk-assign=myproject --strategy=dependency

# Bead operations (delegates to br)
ntm --robot-beads-list --beads-status=open,claimed,in_progress
ntm --robot-bead-claim=br-123 --bead-assignee=agent1
ntm --robot-bead-close=br-123 --bead-close-reason="Completed"
ntm --robot-bead-create --bead-title="..." --bead-priority=1
```

### Comprehensive Diagnosis

```bash
# Full health check + auto-fix where possible (safe fixes only)
ntm --robot-diagnose=myproject --diagnose-fix

# Brief summary only
ntm --robot-diagnose=myproject --diagnose-brief

# One specific pane
ntm --robot-diagnose=myproject --diagnose-pane=5

# Generate support bundle (diagnostic state + logs)
ntm --robot-support-bundle
```

### Context Monitoring

```bash
# Per-pane context window usage
ntm --robot-context=myproject | jq '.panes[] | {pane, context_used_pct, tokens_remaining}'

# Proactive JSONL monitoring for context + provider limits
ntm --robot-monitor=myproject
```

### Schema Introspection

```bash
# Generate JSON Schema for any robot response type
ntm --robot-schema=snapshot
ntm --robot-schema=events
ntm --robot-schema=attention
```

Use `--robot-schema` before parsing any robot response in scripts — the shape is authoritative.

## Transport Availability

Not every surface works on every transport. Before writing a REST client or SSE listener, check:

```bash
ntm --robot-capabilities | jq '.commands[] | {name, transports: [.transports[]?.type]}' | head -40
```

Rough rules of thumb:

- **CLI** — all surfaces (authoritative)
- **REST** — state queries + non-blocking acts; no blocking waits
- **WebSocket/SSE** — event streaming only; no actions
- `--robot-attention` is blocking-by-design; on REST it returns with `retry_after`

## Deprecated / Renamed Flags (Will Silently Misbehave)

| Deprecated | Canonical |
| --- | --- |
| `--assign-beads` | `--beads` |
| `--assign-strategy` | `--strategy` |
| `ntm swarm` | `ntm spawn` |
| `ntm send --distribute` | `ntm --robot-bulk-assign=SESSION --strategy=dependency` |

If your memory/training data says an old flag exists, verify with `--robot-capabilities` before using it. Silent misbehavior is worse than a loud error.

## Known Surface Gotchas

| Gotcha | Symptom | Workaround |
| --- | --- | --- |
| `--robot-health` requires `=<session>` | Silently returns nothing | Always pass session; use `--robot-terse` / `--robot-capabilities` for session-independent checks |
| `--robot-send` does not auto-submit for user pane 1 | Prompt sits un-submitted for pane 1 only | For pane 1, use `ntm send --all --skip-first` (always excludes user pane) OR raw `tmux send-keys ... Enter` |
| `ntm send` without `--skip-first` | Prompt lands in user pane as `zsh: command not found: <prompt>` | Always pass `-s` / `--skip-first` when broadcasting |
| `ntm send` with CASS enabled | Aborts on `Continue anyway? [y/N]` silently in orchestrator loops | Pass `--no-cass-check`, or prefer `--robot-send` |
| `ntm rotate` timeout on wedged CLI | Returns error after 5 min | Skip to `--robot-restart-pane` — it uses `tmux respawn-pane -k` directly |
| `ntm coordinator digest` swallows internal errors | Reports "nothing to report" when a real error happened | Cross-check with `ntm coordinator conflicts` (separate code path) |

## Canonical Operator Loop (one cycle)

```bash
# BOOTSTRAP (once per orchestrator lifecycle)
CURSOR=$(ntm --robot-snapshot | jq -r '.cursor')

while :; do
  # POLL events since last cursor
  EVENTS=$(ntm --robot-events --since-cursor="$CURSOR" --events-actionability=action_required)
  CURSOR=$(echo "$EVENTS" | jq -r '.cursor // empty')
  [ -z "$CURSOR" ] && CURSOR=$(ntm --robot-snapshot | jq -r '.cursor')  # resync on CURSOR_EXPIRED

  # TRIAGE — pick top prioritized item
  TOP=$(echo "$EVENTS" | jq '.items[0] // empty')

  # ACT — send, restart, rotate, escalate (specific per item)
  case "$(echo "$TOP" | jq -r '.family // empty')" in
    stuck_pane)          ntm --robot-smart-restart=myproject --panes="$(echo "$TOP" | jq -r '.pane')" --prompt="$(cat marching_orders.txt)" ;;
    quota_exceeded)      ntm rotate myproject --all-limited ;;
    coord_conflict)      ntm coordinator conflicts myproject ;;
    action_required)     ntm --robot-send=myproject --panes="$(echo "$TOP" | jq -r '.pane')" --msg="..." ;;
    "")                  # no attention — WAIT until next event
                         ntm --robot-wait=myproject --wait-until=attention --attention-cursor="$CURSOR" --timeout=10m ;;
  esac
done
```

This loop replaces ad-hoc bash-grepping of pane buffers. Every step is registry-backed and auditable.
