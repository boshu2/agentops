# Prevention Guide

Stop performance crises before they happen.

## RCH: The Primary Defense

Remote Compilation Helper offloads builds to remote workers, preventing CPU storms.

### Setup RCH

```bash
# 1. Install
curl -fsSL "https://raw.githubusercontent.com/Dicklesworthstone/remote_compilation_helper/main/install.sh" | bash -s -- --easy-mode

# 2. Configure workers (~/.config/rch/workers.toml)
[[workers]]
id = "css"
host = "209.145.54.164"
user = "ubuntu"
identity_file = "~/.ssh/contabo_new_baremetal_superserver_box.pem"
total_slots = 32

[[workers]]
id = "yto"
host = "37.187.75.150"
user = "ubuntu"
identity_file = "~/.ssh/je_ovh_ssh_key.pem"
total_slots = 16

# 3. Start daemon
rch daemon start

# 4. Install hook into Claude Code
rch hook install

# 5. Verify
rch workers probe --all
```

### Auto-Start RCH on Login

```bash
# Add to ~/.zshrc or ~/.bashrc
if command -v rch &>/dev/null && ! rch daemon status --json 2>/dev/null | grep -q '"running": true'; then
  rch daemon start &>/dev/null &
fi
```

### RCH Health Monitoring

```bash
# Add to cron (every 5 minutes)
*/5 * * * * rch daemon status --json | grep -q '"running": false' && rch daemon start
```

---

## Swarm Size Guidelines

More agents != faster development. There's an optimal point.

### Recommended Maximums

| Worker Config | Max Agents | Notes |
|---------------|------------|-------|
| No RCH | 3-5 | Local builds compete with agents |
| RCH + 1 worker (16 slots) | 8-10 | Builds offloaded |
| RCH + 2 workers (48 slots) | 15-20 | Sweet spot for most work |
| RCH + 4 workers (100+ slots) | 25-30 | Diminishing returns beyond this |

### Signs You Have Too Many Agents

- Load average consistently > 1.5x CPU count
- Agents frequently blocked waiting for builds
- File reservation conflicts
- Agent Mail queue backing up

### Optimal Mix

```bash
# General development
ntm spawn project --cc=5 --cod=3 --gmi=2  # 10 agents

# Heavy coding sprint
ntm spawn project --cc=8 --cod=5 --gmi=3  # 16 agents

# Review/polish phase
ntm spawn project --cc=3 --gmi=5  # 8 agents (more review, less coding)
```

---

## Test Timeout Policies

Stuck tests are the #1 cause of resource exhaustion.

### Bun Test Configuration

```json
// bunfig.toml
[test]
timeout = 30000  # 30 seconds per test
bail = 10        # Stop after 10 failures
```

### Cargo Test Configuration

```bash
# In CI/automated environments
cargo test -- --test-threads=4 --timeout 60

# Or via environment
RUST_TEST_THREADS=4 cargo test
```

### Automated Test Killer

```bash
# Add to cron (every hour)
0 * * * * pkill -f 'bun test' --older-than 4h 2>/dev/null
0 * * * * pkill -f 'cargo test' --older-than 4h 2>/dev/null
```

---

## Tmux Session Hygiene

### Auto-Cleanup Old Sessions

```bash
# Add to cron (daily at 3 AM)
0 3 * * * /usr/bin/tmux list-sessions -F '#{session_name} #{session_created}' 2>/dev/null | \
  while read name created; do
    age=$(( ($(date +%s) - created) / 86400 ))
    [[ $age -gt 3 && "$name" =~ ^ntm- ]] && /usr/bin/tmux kill-session -t "$name"
  done
```

### NTM Session Limits

In your workflow, always clean up before spawning:

```bash
# Before spawning new swarm
ntm list | grep -c . > /tmp/session_count
if [[ $(cat /tmp/session_count) -gt 10 ]]; then
  echo "Too many sessions - clean up first"
  ntm list
  exit 1
fi

ntm spawn project --cc=10
```

---

## Resource Monitoring

### Simple Monitoring Script

```bash
#!/bin/bash
# Save as ~/.local/bin/check-resources.sh

LOAD=$(awk '{print $1}' /proc/loadavg)
NPROC=$(nproc)
RATIO=$(awk "BEGIN {printf \"%.2f\", $LOAD / $NPROC}")

if (( $(echo "$RATIO > 2" | bc -l) )); then
  notify-send -u critical "System Overloaded" "Load: ${RATIO}x capacity"
  # Or send to webhook:
  # curl -X POST -d "text=System overloaded: ${RATIO}x" $SLACK_WEBHOOK
fi
```

### Add to Cron

```bash
# Check every 5 minutes
*/5 * * * * ~/.local/bin/check-resources.sh
```

---

## File Descriptor Limits

### Permanent Increase

```bash
# /etc/security/limits.conf
ubuntu soft nofile 65536
ubuntu hard nofile 131072

# /etc/sysctl.conf
fs.file-max = 2097152
```

### Apply Changes

```bash
sudo sysctl -p
# Log out and back in for limits.conf changes
```

---

## Memory Management

### VM Tuning (Prevents the #1 Cause of Session Loss)

> **Incident 2026-02-23:** Default `vfs_cache_pressure=50` on a 499GB btrfs machine caused
> 388GB of filesystem cache + 40GB slab to accumulate. `systemd-oomd` killed the user service,
> destroying all 382 agent sessions. The fix was `vfs_cache_pressure=200` + `min_free_kbytes=2GB`.

**Every new machine must have VM tuning configured:**

```bash
# Check current tuning
sysctl vm.vfs_cache_pressure vm.min_free_kbytes

# Apply (adjust values per machine size — see SKILL.md tuning table)
sudo sysctl -w vm.vfs_cache_pressure=200 vm.min_free_kbytes=2097152

# Persist
sudo tee /etc/sysctl.d/99-system-resource-protection.conf << 'EOF'
vm.swappiness = 10
vm.vfs_cache_pressure = 200
vm.dirty_background_ratio = 5
vm.dirty_ratio = 10
vm.min_free_kbytes = 2097152
fs.inotify.max_user_watches = 524288
fs.inotify.max_user_instances = 1024
vm.max_map_count = 2147483642
EOF
```

**Red flags in existing sysctl configs:**
- `vfs_cache_pressure = 50` — "desktop optimization" scripts set this; it's dangerous on servers
- `MemoryMax=infinity` — in systemd slice overrides, negates your session memory limits
- `min_free_kbytes` under 256MB on machines with 30+ GB RAM

### systemd-oomd Session Protection

Prevent any single session from consuming enough memory to trigger oomd:

```bash
# Per-session limit (prevents one runaway from killing everything)
sudo mkdir -p /etc/systemd/system/session-.scope.d
sudo tee /etc/systemd/system/session-.scope.d/memory-limit.conf << 'EOF'
[Scope]
MemoryMax=64G
MemoryHigh=48G
EOF

# User slice limit (leave headroom for system)
sudo mkdir -p /etc/systemd/system/user-1000.slice.d
sudo tee /etc/systemd/system/user-1000.slice.d/memory-limit.conf << 'EOF'
[Slice]
MemoryMax=460G
MemoryHigh=400G
EOF

sudo systemctl daemon-reload
```

### Swap Configuration

For agent-heavy workloads, ensure adequate swap:

```bash
# Check current swap
swapon --show

# Add swap if needed (16GB example)
sudo fallocate -l 16G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile

# Make permanent
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
```

### Swappiness Tuning

For performance (avoid swapping unless necessary):

```bash
# Temporary
sudo sysctl vm.swappiness=10

# Permanent (in /etc/sysctl.conf)
vm.swappiness = 10
```

---

## Journald Retention for Post-Mortems

When oomd kills sessions, the kill logs can be lost to rotation. Ensure they survive:

```bash
sudo mkdir -p /etc/systemd/journald.conf.d
sudo tee /etc/systemd/journald.conf.d/retain-oomd.conf << 'EOF'
[Journal]
SystemMaxUse=2G
SystemKeepFree=1G
MaxRetentionSec=7day
EOF
sudo systemctl restart systemd-journald
```

---

## Pre-Session Checklist

Before starting a heavy development session:

```bash
# 1. Check system health
./diagnose-system.sh

# 2. Ensure RCH is running
rch daemon status || rch daemon start

# 3. Verify workers online
rch workers probe --all

# 4. Clean stale sessions
/usr/bin/tmux list-sessions | grep ntm- | wc -l  # Should be < 5

# 5. Check available memory
free -h  # Should have > 50GB available

# 6. Ready to work!
ntm spawn project --cc=10 --cod=5
```

---

## Quick Reference

| Problem | Prevention |
|---------|------------|
| CPU storms | Enable RCH |
| Stuck tests | Set test timeouts |
| Memory exhaustion | Limit swarm size |
| **Cache bloat kills all sessions** | **Set vfs_cache_pressure=200, min_free_kbytes=2GB** |
| **oomd kills user service** | **Set per-session MemoryMax=64G via systemd** |
| **Lost oomd post-mortem logs** | **Set journald SystemMaxUse=2G, MaxRetentionSec=7day** |
| FD exhaustion | Increase limits |
| Session sprawl | Auto-cleanup cron |
| Build conflicts | Use RCH deduplication |
