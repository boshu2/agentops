# Escalation Guide

When standard remediation isn't enough, follow this escalation ladder.

## Level 1: Standard Cleanup (Agent-Safe)

**Trigger:** Load ratio > 1.5x, memory > 80% used

**Actions:**
```bash
# Kill stuck tests
pkill -f 'bun test' --older-than 12h
pkill -f 'cargo test' --older-than 12h

# Kill abandoned dev servers
pkill -f 'next dev' --older-than 24h
pkill -f 'bun --hot' --older-than 24h

# Clean test tmux sessions
/usr/bin/tmux list-sessions -F '#{session_name}' | grep -E '^ntm-(test|lifecycle|rapid)-' | \
  xargs -I{} /usr/bin/tmux kill-session -t {}
```

**Expected Impact:** 20-40% load reduction

---

## Level 2: Aggressive Cleanup (Requires Caution)

**Trigger:** Load ratio > 2x, Level 1 ineffective

**Actions:**
```bash
# Kill ALL bun/cargo tests regardless of age
pkill -9 -f 'bun test'
pkill -9 -f 'cargo test'

# Kill old agent sessions (24+ hours)
for pid in $(ps -eo pid,etimes,comm | awk '$2 > 86400 && $3 ~ /claude|codex/ {print $1}'); do
  echo "Killing old agent: $pid"
  kill $pid
done

# Kill all ntm sessions except current work
CURRENT_SESSION=$(tmux display-message -p '#S' 2>/dev/null || echo "")
/usr/bin/tmux list-sessions -F '#{session_name}' | while read session; do
  [[ "$session" != "$CURRENT_SESSION" ]] && /usr/bin/tmux kill-session -t "$session"
done
```

**Expected Impact:** 50-70% load reduction

---

## Level 3: Emergency Triage (User Approval Required)

**Trigger:** Load ratio > 3x, system unresponsive

**REQUIRES EXPLICIT USER APPROVAL per AGENTS.md Rule 1**

**Actions:**
```bash
# Kill ALL agent sessions except current one
MY_PID=$$
for pid in $(pgrep -f 'claude|codex|gemini'); do
  [[ $pid != $MY_PID ]] && kill -9 $pid
done

# Kill all tmux sessions
/usr/bin/tmux kill-server 2>/dev/null || true

# Kill all rustc processes (compilation storm)
pkill -9 rustc

# Kill memory hogs (> 10GB)
ps -eo pid,rss,comm --sort=-rss | awk '$2 > 10485760 {print $1}' | head -5 | xargs kill -9
```

**Expected Impact:** 80-95% load reduction

---

## Level 4: System Recovery (Last Resort)

**Trigger:** OOM killer active, kernel panics, total unresponsiveness

**From Remote SSH:**
```bash
# Sync filesystems
sync

# Drop caches
echo 3 | sudo tee /proc/sys/vm/drop_caches

# Kill biggest memory consumer
kill -9 $(ps -eo pid --sort=-rss | head -2 | tail -1)
```

**From Physical Console (Magic SysRq):**
```
Alt+SysRq+R  # Take keyboard from X
Alt+SysRq+E  # SIGTERM to all except init
Alt+SysRq+I  # SIGKILL to all except init
Alt+SysRq+S  # Sync filesystems
Alt+SysRq+U  # Remount read-only
Alt+SysRq+B  # Reboot
```

Memory: **R E I S U B** = "Reboot Even If System Utterly Broken"

---

## Level 5: systemd-oomd Killed Everything (Post-Mortem)

**Trigger:** All sessions died simultaneously. `systemctl --user status user@1000.service` shows
"Failed with result 'signal'" and `code=killed, status=9/KILL`.

This means `systemd-oomd` killed the user service manager, which cascaded to kill wezterm-mux-server
and all sessions.

**Investigation:**

```bash
# 1. Confirm oomd was the culprit
journalctl --since "6 hours ago" | grep -iE "oom|kill|wezterm|user@1000" | tail -30

# 2. Check what the memory peak was
systemctl show user-1000.slice -p MemoryPeak
# If this was near EffectiveMemoryMax → oomd killed it

# 3. Check per-session memory peaks (find the offender)
for scope in $(journalctl --since "6 hours ago" | grep -oP 'session-\d+\.scope' | sort -u); do
  echo "$scope: $(journalctl --since '6 hours ago' | grep "$scope" | grep -oP 'memory peak: \S+')"
done

# 4. Check if resource-watchdog was running (it should have prevented this)
systemctl --user status resource-watchdog.service
```

**Prevention (apply after recovery):**

```bash
# Set per-session memory limits
sudo mkdir -p /etc/systemd/system/session-.scope.d
sudo tee /etc/systemd/system/session-.scope.d/memory-limit.conf << 'EOF'
[Scope]
MemoryMax=64G
MemoryHigh=48G
EOF

# Fix VM tuning to prevent cache bloat (the common trigger)
sudo sysctl -w vm.vfs_cache_pressure=200 vm.min_free_kbytes=2097152

# Retain oomd logs for next time
sudo mkdir -p /etc/systemd/journald.conf.d
sudo tee /etc/systemd/journald.conf.d/retain-oomd.conf << 'EOF'
[Journal]
SystemMaxUse=2G
MaxRetentionSec=7day
EOF

sudo systemctl daemon-reload
sudo systemctl restart systemd-journald
```

---

## Post-Recovery Checklist

After any escalation:

1. **Verify system stable**
   ```bash
   uptime && free -h && df -h && cat /proc/pressure/memory
   ```

2. **Check for data loss**
   ```bash
   git status  # In all active project directories
   ```

3. **Restart essential services**
   ```bash
   rch daemon start
   ```

4. **Check VM tuning is correct**
   ```bash
   sysctl vm.vfs_cache_pressure vm.min_free_kbytes
   # Should be: vfs >= 150 (200 for btrfs), min_free >= 256MB
   ```

5. **Check session memory limits are in place**
   ```bash
   ls /etc/systemd/system/session-.scope.d/memory-limit.conf
   ```

6. **Document incident**
   - What triggered it?
   - What was killed?
   - Was work lost?

7. **Prevention**
   - Fix VM tuning (vfs_cache_pressure, min_free_kbytes)
   - Set per-session MemoryMax via systemd
   - Reduce swarm size if sessions are consuming 100+ GB each
   - Enable RCH for build offloading
   - Set up monitoring alerts
