# Wezterm Terminal Recovery

When wezterm-mux-server becomes unresponsive (connection timeouts, "Checking server version" hangs), you can save running agent sessions by migrating them to tmux using `reptyr -T` (terminal stealing mode).

## Symptoms

- SSH multiplexing via wezterm times out
- `wezterm cli list` hangs or fails
- Mux server log shows repeated "Broken pipe" errors
- Many agent sessions (claude, codex) would be lost if wezterm restarts

---

## Diagnose Wezterm State

```bash
# Check wezterm processes
ps aux | grep wezterm | grep -v grep

# Check mux server socket
ls -la /run/user/$(id -u)/wezterm/

# View mux server log for errors
cat /run/user/$(id -u)/wezterm/wezterm-mux-server-log-*.txt | tail -20

# Kill stuck proxy processes (safe, won't affect sessions)
pkill -f 'wezterm cli.*proxy'
```

---

## Migrate Sessions to tmux with reptyr

### Prerequisites

```bash
# Install reptyr if missing
sudo apt-get install -y reptyr

# Enable ptrace attachment (required for reptyr)
sudo sysctl -w kernel.yama.ptrace_scope=0

# Create tmux session to hold migrated processes
tmux new-session -d -s agents -x 200 -y 50
```

### Migration Script

```bash
# Get all claude/codex PIDs
PIDS=$(ps -eo pid,args | grep -E 'claude --dangerously|codex --dangerously' | grep -v grep | awk '{print $1}')

# Migrate each to a tmux window
for pid in $PIDS; do
  echo "Migrating PID $pid..."
  tmux new-window -t agents -n "agent-$pid"
  tmux send-keys -t "agents:agent-$pid" "reptyr -T $pid" Enter
  sleep 0.5
done

echo "Migration complete. Attach with: tmux attach -t agents"
```

---

## Verify Migration Success

```bash
# Check which sessions attached successfully
for win in $(tmux list-windows -t agents -F "#{window_name}" | grep agent); do
  content=$(tmux capture-pane -t "agents:$win" -p 2>/dev/null | tail -10)
  if echo "$content" | grep -qE "bypass|Baked|Claude|codex"; then
    echo "✓ $win ATTACHED"
  elif echo "$content" | grep -q "Unable to attach"; then
    echo "✗ $win FAILED"
  else
    echo "? $win UNKNOWN - check manually"
  fi
done
```

---

## View All Migrated Sessions

```bash
# Attach to tmux and browse interactively
tmux attach -t agents

# Navigation:
# Ctrl-b w     - visual window selector (BEST)
# Ctrl-b n/p   - next/previous window
# Ctrl-b 0-9   - jump to window by number
```

---

## reptyr Failure Modes

| Error | Cause | Solution |
|-------|-------|----------|
| "Operation not permitted" | ptrace blocked | `sudo sysctl -w kernel.yama.ptrace_scope=0` |
| "shares process group" | Process has children | Use `reptyr -T` instead of `reptyr` |
| "Unable to attach" even with sudo | Process has additional protections | Cannot migrate; will be lost |

---

## After Migration — Safe to Restart Wezterm

```bash
# Kill the old mux server
pkill -9 wezterm-mux-server

# Wezterm will auto-start a fresh mux on next connection
# Your migrated sessions remain safe in tmux
```

---

## Restore ptrace Security

```bash
# After migration, restore ptrace restrictions
sudo sysctl -w kernel.yama.ptrace_scope=1
```

**Success Rate:** Typically 50-70% of sessions can be migrated. Actively running processes with complex subprocess trees are harder to migrate. Older, idle sessions migrate more reliably.

---

## Complete Scenario: Wezterm Mux Unresponsive with Active Agent Swarm

When wezterm connection times out but you have 20+ agent sessions you don't want to lose:

```bash
# 1. Connect via plain SSH (bypasses wezterm mux)
ssh user@host

# 2. Check how many sessions would be lost
ps -eo pid,tty,args | grep -E 'claude|codex' | grep -v grep | wc -l

# 3. Install reptyr and enable ptrace
sudo apt-get install -y reptyr
sudo sysctl -w kernel.yama.ptrace_scope=0

# 4. Create tmux rescue session
tmux new-session -d -s agents -x 200 -y 50

# 5. Migrate all agent sessions
for pid in $(ps -eo pid,args | grep -E 'claude --dangerously|codex --dangerously' | grep -v grep | awk '{print $1}'); do
  tmux new-window -t agents -n "agent-$pid"
  tmux send-keys -t "agents:agent-$pid" "reptyr -T $pid" Enter
  sleep 0.5
done

# 6. Verify what was saved
for win in $(tmux list-windows -t agents -F "#{window_name}" | grep agent); do
  tmux capture-pane -t "agents:$win" -p | grep -qE "bypass|Claude" && echo "✓ $win" || echo "✗ $win"
done

# 7. Kill stuck wezterm (safe now)
pkill -f 'wezterm cli.*proxy'
pkill -9 wezterm-mux-server

# 8. Restore security and attach
sudo sysctl -w kernel.yama.ptrace_scope=1
tmux attach -t agents  # Use Ctrl-b w to browse windows
```
