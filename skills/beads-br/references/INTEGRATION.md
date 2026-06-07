# br Integration Patterns

## bv (Beads Viewer) Integration

bv is a graph-aware triage engine for beads.

**CRITICAL:** Never run bare `bv` — it launches interactive TUI and blocks the session.

```bash
# Always use --robot-* flags:
bv --robot-triage        # Full triage with recommendations
bv --robot-next          # Single top pick
bv --robot-plan          # Parallel execution tracks
bv --robot-insights      # Graph metrics (PageRank, cycles, etc.)
```

### Check Graph Health

```bash
bv --robot-insights | jq '.Cycles'       # Must be empty
bv --robot-insights | jq '.bottlenecks'  # Find blocking issues
```

---

## MCP Agent Mail Integration

Use bead IDs as coordination threads for multi-agent work:

### Mapping Cheat Sheet

| Concept | Value |
|---------|-------|
| Mail `thread_id` | `bd-###` |
| Mail subject | `[bd-###] ...` |
| File reservation `reason` | `bd-###` |
| Commit messages | Include `bd-###` for traceability |

### Agent Mail Workflow

```python
# 1. Reserve files for bead
file_reservation_paths(..., reason="bd-123")

# 2. Announce work in thread
send_message(..., thread_id="bd-123", subject="[bd-123] Starting...")

# 3. Do work...

# 4. Close bead when done
br close bd-123 --reason "Completed"

# 5. Release reservations
release_file_reservations(...)
```

---

## Multi-Agent Coordination

When multiple agents work on the same project:

1. **Use Agent Mail file reservations** to avoid conflicts
2. **Use bead ID as thread_id** for communication
3. **Check `br ready --json`** to see unblocked work
4. **Close beads when done** to unblock dependents

### Finding Parallel Work

```bash
# Get parallel execution tracks
bv --robot-plan

# Multiple agents can work on independent branches of the dependency graph
```

---

## Standard Agent Workflow

```bash
# 1. Initialize (one-time per project)
cd my-project
br init

# 2. Find work
br ready --json

# 3. Claim work
br update bd-abc123 --status in_progress --assignee "$(git config user.email)"

# 4. Do work...

# 5. Complete
br close bd-abc123 --reason "Implemented feature X"

# 6. Sync to git
br sync --flush-only
git add .beads/
git commit -m "feat: implement X (bd-abc123)"
```

---

## Session Ending Pattern

Before ending any session:

```bash
git pull --rebase
br sync --flush-only
git add .beads/ && git commit -m "Update issues"
git push
git status  # MUST show "up to date with origin"
```

---

## Creating Good Beads

```bash
br create "Title that explains the task" \
  --type task \
  --priority 1 \
  --description "Detailed description with acceptance criteria"
```

Include in descriptions:
- Clear scope
- Acceptance criteria
- Dependencies (add separately via `br dep add`)
- Context for "future self"

---

## Differences from bd (Go beads)

| Aspect | br (Rust) | bd (Go) |
|--------|-----------|---------|
| Git operations | **Never** (explicit) | Auto-commit, hooks |
| Storage | SQLite + JSONL | Dolt/SQLite |
| Background daemon | **No** | Yes |
| Hook installation | **Manual** | Automatic |
| Complexity | Focused | Feature-rich |

### What br Does NOT Support (by design)

- Automatic git commits
- Git hook installation
- Background daemon/RPC
- Dolt backend
- Linear/Jira sync
- Web UI (use bv for TUI)
- Multi-repo sync
- Real-time collaboration
