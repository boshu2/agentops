---
name: crank
description: >
  Fully autonomous epic execution. Runs until ALL children are CLOSED.
  Auto-detects role: Mayor uses polecats for parallel execution,
  Crew executes sequentially via /implement. NO human prompts, NO stopping.
version: 2.1.0
context: fork
triggers:
  - "crank"
  - "crank this epic"
  - "run autonomously"
  - "execute to completion"
  - "crank it"
  - "full auto"
  - "run unattended"
allowed-tools: Bash, Read, Glob, Grep, TodoWrite, Task
skills:
  - beads
  - gastown
  - implement
---

# crank: Autonomous Epic Execution

> **Runs until epic is CLOSED. Auto-adapts to Mayor (parallel) or Crew (sequential) mode.**

## Role Detection

Crank auto-detects execution mode based on current context:

```bash
# Check role by directory structure
if [[ "$PWD" == */mayor/* ]] || [[ "$PWD" == ~/gt ]]; then
    ROLE="mayor"    # Can spawn polecats
else
    ROLE="crew"     # Execute directly
fi
```

| Role | Execution Style | Parallelism | Command |
|------|-----------------|-------------|---------|
| **Mayor** | Dispatch to polecats via `gt sling` | Up to 8 concurrent | `gt sling <issue> <rig>` |
| **Crew** | Execute directly via `/implement` | Sequential | `/implement <issue>` |

## Quick Start

```bash
# From anywhere (auto-detects role)
/crank <epic-id>

# Force a specific mode
/crank <epic-id> --mode=crew     # Sequential, no polecats
/crank <epic-id> --mode=mayor    # Parallel via polecats
```

---

## Context Inference

When `/crank` is invoked without an epic-id, check the preceding conversation for context:

### Priority Order

1. **Explicit epic-id** - If user provides an epic ID, use it
2. **Recently discussed epic** - If an epic was mentioned in conversation, use it
3. **Hooked work** - Check `gt hook` for assigned epic
4. **In-progress epic** - Check `bd list --type=epic --status=in_progress`
5. **Ask user** - If no context found, ask which epic to crank

### Detection Logic

```markdown
## On Invocation Without Epic ID

1. Scan conversation for epic references:
   - Look for issue IDs with epic type (e.g., "ap-68ohb")
   - Check for "epic", "parent issue" mentions
   - Extract from recent bd commands in conversation

2. Check Gas Town hook:
   ```bash
   gt hook  # Returns hooked work including parent epic
   ```

3. Check beads state:
   ```bash
   bd list --type=epic --status=in_progress | head -1
   ```

4. If nothing found, ask:
   "Which epic should I crank? Run `bd list --type=epic` to see available epics."
```

### Example

```
User: let's work on ap-68ohb - it has 27 children to implement
User: [does some planning work]
User: /crank

→ Crank infers epic from conversation: ap-68ohb
→ Starts autonomous execution without requiring re-specification
```

---

## The ODMCR Loop

Both modes use the same reconciliation loop, just different dispatch mechanisms:

```
┌─────────────────────────────────────────────────────────┐
│                    ODMCR LOOP                           │
│                                                         │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐          │
│  │ OBSERVE  │───►│ DISPATCH │───►│ MONITOR  │          │
│  └──────────┘    └──────────┘    └──────────┘          │
│       ▲                               │                 │
│       │          ┌──────────┐         │                 │
│       │          │  RETRY   │◄────────┤                 │
│       │          └──────────┘         │                 │
│       │               │               ▼                 │
│       │          ┌──────────┐    ┌──────────┐          │
│       └──────────│ (loop)   │◄───│ COLLECT  │          │
│                  └──────────┘    └──────────┘          │
│                                                         │
│  EXIT: All children status=closed                       │
└─────────────────────────────────────────────────────────┘
```

---

## Crew Mode Execution

When running as crew (in a crew workspace), crank executes issues sequentially:

### Crew OBSERVE
```bash
# Same as mayor - query epic state
bd ready --parent=<epic>
bd list --parent=<epic> --status=in_progress
bd list --parent=<epic> --status=closed
```

### Crew DISPATCH
```bash
# Execute directly, one at a time
issue=$(bd ready --parent=<epic> --limit=1 | awk '{print $1}')
/implement $issue
```

**Key difference**: No polecats, no convoy. Just sequential `/implement` calls.

### Crew MONITOR
```bash
# Check if current issue completed
bd show <issue> | grep "CLOSED"
```

### Crew COLLECT
```bash
# Verify completion
git status                    # Check for uncommitted work
bd sync                       # Sync beads
git add . && git commit       # If needed
```

### Crew RETRY
```bash
# On failure, retry the same issue
# After MAX_RETRIES, skip and mail escalation
```

### Crew Execution Flow

```python
def crank_crew(epic_id):
    """Sequential execution for crew context."""

    retry_counts = {}

    while not epic_complete(epic_id):
        # OBSERVE
        ready = bd_ready(parent=epic_id, limit=1)
        if not ready:
            # All remaining are blocked or in_progress elsewhere
            sleep(30)
            continue

        issue = ready[0]

        # DISPATCH (sequential - execute directly)
        bd_update(issue, status='in_progress')
        result = implement(issue)  # Calls /implement skill

        # COLLECT
        if result.success:
            bd_close(issue, reason=result.summary)
            git_commit_and_push()
        else:
            # RETRY
            retry_counts[issue] = retry_counts.get(issue, 0) + 1
            if retry_counts[issue] >= MAX_RETRIES:
                bd_update(issue, status='blocked',
                         notes=f"Failed after {MAX_RETRIES} attempts")
                mail_escalation(issue)
            else:
                # Will retry on next loop iteration
                pass

    # Complete
    bd_close(epic_id, reason="All children completed via crank (crew mode)")
```

---

## Mayor Mode Execution

When running as mayor (at town root or in mayor/), crank dispatches to polecats:

### Mayor DISPATCH
```bash
# Parallel dispatch to polecats
READY=$(bd ready --parent=<epic> | awk '{print $1}')
IN_PROGRESS=$(bd list --parent=<epic> --status=in_progress | wc -l)
SLOTS=$((MAX_POLECATS - IN_PROGRESS))

echo "$READY" | head -$SLOTS | while read issue; do
    gt sling "$issue" <rig>
done
```

### Mayor MONITOR
```bash
# Poll convoy status (low-token)
gt convoy list | grep <epic-prefix>
gt convoy status <convoy-id>
```

### Mayor Execution Flow

```python
def crank_mayor(epic_id, rig):
    """Parallel execution via polecats."""

    retry_counts = {}

    while not epic_complete(epic_id):
        # OBSERVE
        ready = bd_ready(parent=epic_id)
        in_progress = bd_list(parent=epic_id, status='in_progress')

        # DISPATCH (parallel to polecats)
        slots = MAX_POLECATS - len(in_progress)
        for issue in ready[:slots]:
            if retry_counts.get(issue, 0) < MAX_RETRIES:
                gt_sling(issue, rig)

        # MONITOR
        sleep(POLL_INTERVAL)
        convoy_status = gt_convoy_status()

        # COLLECT
        for completed in convoy_status.completed:
            verify_completion(completed)

        # RETRY
        for failed in convoy_status.failed:
            retry_counts[failed] = retry_counts.get(failed, 0) + 1
            if retry_counts[failed] >= MAX_RETRIES:
                handle_exhausted(failed)
            else:
                schedule_retry(failed, backoff=30 * 2**retry_counts[failed])

    # Complete
    bd_close(epic_id, reason="All children completed via crank (mayor mode)")
    mail_human(f"Epic {epic_id} completed")
```

---

## Mode Comparison

| Aspect | Crew Mode | Mayor Mode |
|--------|-----------|------------|
| **Parallelism** | Sequential (1 at a time) | Up to 8 concurrent |
| **Dispatch** | `/implement` directly | `gt sling` to polecats |
| **Context** | Uses current session | Spawns new sessions |
| **Speed** | Slower but simpler | Faster with parallelism |
| **Monitoring** | Inline (same session) | Convoy dashboard |
| **Best for** | Small epics, testing | Large epics, overnight |

---

## Arguments

| Arg | Purpose | Default |
|-----|---------|---------|
| `<epic-id>` | Epic to execute | Required |
| `--mode` | Force `crew` or `mayor` | Auto-detect |
| `--max` | Max concurrent polecats (mayor only) | 8 |
| `--dry-run` | Preview waves without executing | false |
| `status` | Show current crank progress | - |
| `stop` | Graceful stop at next checkpoint | - |

---

## Constants

| Constant | Default | Description |
|----------|---------|-------------|
| `MAX_POLECATS` | 8 | Max concurrent workers (mayor) |
| `POLL_INTERVAL` | 30s | Seconds between status checks |
| `MAX_RETRIES` | 3 | Retries before escalation |
| `BACKOFF_BASE` | 30s | Base for exponential backoff |

---

## Prerequisites

Before cranking:

```bash
# 1. Epic exists with children
bd show <epic-id>

# 2. At least one issue is ready
bd ready --parent=<epic>

# 3. If mayor mode: rig exists
gt rig list
```

---

## Example Sessions

### Crew Mode
```bash
> /crank ap-68ohb

[CRANK] Detected role: crew (sequential mode)
[CRANK] Starting autonomous execution of ap-68ohb
[OBSERVE] Epic ap-68ohb: 27 children, 0 closed, 27 ready
[DISPATCH] Implementing ap-68ohb.2 directly...
[IMPLEMENT] E2E: Gateway LLM proxy streaming and non-streaming
... implementation happens ...
[COLLECT] ap-68ohb.2 completed
[DISPATCH] Implementing ap-68ohb.4 directly...
... continues sequentially ...
[COMPLETE] Epic ap-68ohb finished. 27/27 children closed.
```

### Mayor Mode
```bash
> /crank ap-68ohb --mode=mayor

[CRANK] Running in mayor mode (parallel via polecats)
[CRANK] Target rig: ai_platform
[OBSERVE] Epic ap-68ohb: 27 children, 0 closed, 27 ready
[DISPATCH] Slinging ap-68ohb.2, .4, .5, .6, .7, .8, .9, .10 to ai_platform
[MONITOR] Convoy cv-xyz active, 8 polecats working
[MONITOR] ... (30s) ...
[COLLECT] ap-68ohb.2 completed, verified
... continues until all closed ...
[COMPLETE] Epic ap-68ohb finished. 27/27 children closed.
```

---

## Safety Rails

Even in full-auto mode, crank respects:

1. **Retry limits**: Issues don't retry forever
2. **Blocker escalation**: Humans notified of stuck work
3. **No force operations**: Never `--force` on git
4. **Beads integrity**: Always `bd sync` before state changes
5. **Commit after each issue (crew)**: Progress is never lost

---

## Session Recovery

If session ends mid-crank:

```bash
# State is recoverable from beads
bd list --parent=<epic> --status=in_progress  # What was running
bd list --parent=<epic> --status=closed       # What completed

# Resume with
/crank <epic>  # Picks up where it left off
```

---

## Related

- [odmcr.md](odmcr.md) - Detailed ODMCR loop specification
- [failure-taxonomy.md](failure-taxonomy.md) - Failure types and handling
- `/implement` - Single issue execution (used by crew mode)
- `/implement-wave` - Single wave execution with validation
