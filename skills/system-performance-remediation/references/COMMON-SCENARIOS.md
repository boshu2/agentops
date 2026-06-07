# Common Scenarios

Step-by-step remediation for common performance situations.

---

## Scenario: Load Average > 100 on 64-Core Machine

```bash
# 1. Quick assessment
ps aux --sort=-%cpu | head -20

# 2. Count process types
ps -eo comm | sort | uniq -c | sort -rn | head -10

# 3. Kill stuck tests first (safest, 12+ hours)
ps -eo pid,etimes,args | grep 'bun test' | awk '$2 > 43200 {print $1}' | xargs -r kill -9

# 4. Kill abandoned dev servers (24+ hours)
ps -eo pid,etimes,args | grep 'next dev' | awk '$2 > 86400 {print $1}' | xargs -r kill

# 5. Clean tmux test sessions
/usr/bin/tmux list-sessions -F '#{session_name}' | grep -E '^ntm-(test|lifecycle|rapid)-' | xargs -I{} /usr/bin/tmux kill-session -t {}

# 6. Verify improvement
sleep 10 && uptime
```

---

## Scenario: Agent Swarm Meltdown (Competing Builds)

When you have 20+ agents all building the same Rust project:

```bash
# 1. Check for competing duckdb builds (the smoking gun)
ps aux | grep cc1plus | grep -oP 'target[^/]*/' | sort | uniq -c
# If multiple targets → agents are thrashing each other

# 2. Check CPU pressure (load can look "ok" while system is sluggish)
cat /proc/pressure/cpu | head -1
# some avg10=57.18 → 57% of time tasks waiting for CPU = BAD

# 3. Find duplicate cargo check --all-targets
ps -eo pid,etimes,args | grep 'cargo check --all-targets' | grep -v grep

# 4. DON'T just kill the cargo builds - they'll respawn!
# Instead, find and kill the confused OLD agents spawning them
ps -eo pid,etimes,pcpu,args | grep -E 'claude --dangerously|codex --dangerously' | \
  grep -v grep | awk '$2 > 57600 {print $1, int($2/3600)"h", $3"%"}' | \
  while read pid age cpu; do
    echo "Killing old agent $pid ($age, $cpu CPU)"
    kill $pid
  done

# 5. Renice remaining compilation to lowest priority
for pid in $(pgrep -f '/bin/cargo') $(pgrep cc1plus); do
  renice 19 -p $pid 2>/dev/null
  ionice -c 3 -p $pid 2>/dev/null
done

# 6. Monitor improvement
for i in 1 2 3; do
  sleep 10
  echo "$(date +%H:%M:%S) Load: $(cat /proc/loadavg | awk '{print $1}') | CPU pressure: $(cat /proc/pressure/cpu | awk -F= '{print $2}' | cut -d' ' -f1)%"
done
```

---

## Scenario: Whack-a-Mole (Processes Keep Respawning)

When you kill a process and it immediately comes back:

```bash
# 1. You're killing the symptom, not the cause
# Find the PARENT that keeps respawning the process
CHILD_PID=489714  # The process that keeps coming back
PARENT_PID=$(ps -o ppid= -p $CHILD_PID | tr -d ' ')
ps -o pid,args -p $PARENT_PID

# 2. If parent is an agent (claude/codex), that's your real target
# Check if it's an old confused agent
ps -o pid,etimes,args -p $PARENT_PID

# 3. Kill the agent, not its children
kill $PARENT_PID

# 4. The respawning will stop
```

---

## Scenario: "Too Many Open Files" Errors

```bash
# 1. Find FD hogs
lsof 2>/dev/null | awk '{print $1, $2}' | sort | uniq -c | sort -rn | head -10

# 2. Kill the biggest offender if it's safe (e.g., old test runner)
# [Identify safe targets first]

# 3. Temporary relief
ulimit -n 65536
```

---

## Scenario: System Totally Unresponsive

```bash
# Use magic SysRq if available (last resort)
# Alt+SysRq+F = OOM killer (kills biggest memory hog)

# From remote SSH, kill obvious targets
pkill -9 -f 'bun test'
pkill -9 -f 'cargo test'

# Kill all agent sessions except the one you're in
MY_PID=$$
for pid in $(pgrep -f 'claude|codex|gemini'); do
  [[ $pid != $MY_PID ]] && kill -9 $pid
done
```

---

## Scenario: Wezterm Mux Unresponsive with Active Agent Swarm

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

See [WEZTERM-RECOVERY.md](WEZTERM-RECOVERY.md) for full details.

---

## Scenario: Emergency Mass Cleanup (REQUIRES USER APPROVAL)

**REQUIRES EXPLICIT USER APPROVAL** per AGENTS.md Rule 1.

```bash
# Step 1: Kill zombies (always safe)
# (zombies can't be killed directly - signal their parents)

# Step 2: Kill all stuck bun tests (12+ hours)
pgrep -f 'bun test' | while read pid; do
  age=$(ps -o etimes= -p $pid 2>/dev/null | tr -d ' ')
  [[ "$age" -gt 43200 ]] && kill -9 $pid && echo "Killed bun test $pid (${age}s old)"
done

# Step 3: Kill old dev servers (24+ hours, <1% CPU)
ps -eo pid,etimes,pcpu,args | grep -E 'next dev|bun --hot|vite' | while read pid age cpu cmd; do
  [[ "$age" -gt 86400 && "${cpu%.*}" -lt 1 ]] && kill $pid && echo "Killed dev server $pid"
done

# Step 4: Verify improvement
sleep 5 && uptime
```

---

## Scenario: RCH Not Working (Builds Running Locally)

```bash
# 1. Is daemon running?
rch daemon status --json 2>/dev/null | head -5

# 2. Are workers reachable?
rch workers probe --all

# 3. Test hook
echo '{"tool_name":"Bash","tool_input":{"command":"cargo check --version"}}' | /usr/local/bin/rch 2>&1 | head -20

# 4. Quick fix if broken
pkill -9 rchd; sleep 1; rch daemon start && rch workers probe --all
```

See [RCH-INTEGRATION.md](RCH-INTEGRATION.md) for full troubleshooting.
