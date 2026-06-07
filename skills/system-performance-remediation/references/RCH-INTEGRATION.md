# RCH Integration for System Performance

Remote Compilation Helper is your primary defense against compilation-induced system overload.

## Why RCH Matters for Performance

Running 15+ AI coding agents creates **compilation storms**:
- Each `cargo build` can consume 100% of multiple cores
- Concurrent builds compete for CPU, memory, and I/O
- System becomes unresponsive, agents timeout, work is lost

RCH transparently offloads compilation to remote workers:
- Agents think compilation ran locally
- Your workstation stays responsive
- Builds complete faster on powerful workers

---

## Quick Setup

```bash
# 1. Install (includes daemon, hook, workers)
curl -fsSL "https://raw.githubusercontent.com/Dicklesworthstone/remote_compilation_helper/main/install.sh" | bash -s -- --easy-mode

# 2. Configure at least one worker
cat >> ~/.config/rch/workers.toml << 'EOF'
[[workers]]
id = "css"
host = "209.145.54.164"
user = "ubuntu"
identity_file = "~/.ssh/contabo_new_baremetal_superserver_box.pem"
total_slots = 32
priority = 100
EOF

# 3. Start daemon
rch daemon start

# 4. Install Claude Code hook
rch hook install

# 5. Verify
rch workers probe --all
```

---

## Diagnosis When Builds Are Slow

### Step 1: Is RCH Running?

```bash
rch daemon status --json
```

**Expected:** `"running": true`

**If not running:**
```bash
rch daemon start
```

### Step 2: Are Workers Reachable?

```bash
rch workers probe --all
```

**Expected:** All workers show "OK" with latency

**If workers unreachable:**
```bash
# Test SSH directly
ssh -i ~/.ssh/your_key.pem ubuntu@worker_ip echo "OK"

# Check worker has Rust
ssh -i ~/.ssh/your_key.pem ubuntu@worker_ip "cargo --version"
```

### Step 3: Is Hook Intercepting?

```bash
echo '{"tool_name":"Bash","tool_input":{"command":"cargo build"}}' | /usr/local/bin/rch 2>&1
```

**Expected:** "Selected worker:", "Remote compilation succeeded"

**If falling back to local:**
- Check socket path matches: `rch config show | grep socket` vs `rch daemon status --json | grep socket`
- Check confidence threshold: may be too high for some commands

### Step 4: Full Diagnostic

```bash
/home/ubuntu/.claude/skills/rch/scripts/diagnose-rch.sh
```

---

## Common Issues

### Socket Path Mismatch

**Symptom:** Hook falls back to local, daemon is running

**Diagnosis:**
```bash
config_socket=$(grep socket_path ~/.config/rch/config.toml 2>/dev/null | cut -d'"' -f2)
daemon_socket=$(rch daemon status --json 2>/dev/null | grep -o '"socket_path": "[^"]*"' | cut -d'"' -f4)
echo "Config: $config_socket"
echo "Daemon: $daemon_socket"
```

**Fix:**
```bash
# Ensure config.toml has correct socket path
cat >> ~/.config/rch/config.toml << 'EOF'
[general]
socket_path = "/tmp/rch.sock"
EOF

# Restart daemon
pkill -9 rchd; sleep 1; rch daemon start
```

### Worker Missing Rust

**Symptom:** "rustup: not found" or "cargo: not found"

**Fix on worker:**
```bash
ssh -i ~/.ssh/key.pem ubuntu@worker_ip
curl -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain nightly
source ~/.cargo/env
```

### Visibility Set to None

**Symptom:** `rch status` shows nothing, but daemon is running

**Fix:**
```bash
# Add to ~/.config/rch/config.toml
[output]
visibility = "normal"
```

---

## Performance Tuning

### Worker Selection Algorithm

RCH scores workers by:
```
Score = SpeedScore × AvailabilityFactor × AffinityBonus × PriorityWeight
```

Optimize by:
- **Setting priority:** Higher priority workers get more work
- **Benchmarking:** `rch workers benchmark` updates speed scores
- **Affinity:** Workers with cached projects get bonus (incremental builds)

### Multiple Workers

Distribute load across multiple machines:

```toml
# ~/.config/rch/workers.toml

[[workers]]
id = "css"
host = "209.145.54.164"
user = "ubuntu"
identity_file = "~/.ssh/contabo_new_baremetal_superserver_box.pem"
total_slots = 32
priority = 100

[[workers]]
id = "yto"
host = "37.187.75.150"
user = "ubuntu"
identity_file = "~/.ssh/je_ovh_ssh_key.pem"
total_slots = 16
priority = 80

[[workers]]
id = "fmd"
host = "51.222.245.56"
user = "ubuntu"
identity_file = "~/.ssh/je_ovh_ssh_key.pem"
total_slots = 16
priority = 80
```

### Compression Tuning

For fast networks (< 10ms latency):
```toml
[transfer]
compression_level = 1  # Faster compression
```

For slow networks (> 50ms latency):
```toml
[transfer]
compression_level = 6  # Better ratio
```

---

## Auto-Start Configuration

### User Systemd Service

```bash
mkdir -p ~/.config/systemd/user

cat > ~/.config/systemd/user/rchd.service << 'EOF'
[Unit]
Description=RCH Remote Compilation Daemon
After=network.target

[Service]
Type=simple
ExecStart=/home/ubuntu/.local/bin/rchd
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
EOF

systemctl --user daemon-reload
systemctl --user enable --now rchd
```

### Shell Integration

```bash
# Add to ~/.zshrc
if command -v rch &>/dev/null; then
  rch daemon status --json 2>/dev/null | grep -q '"running": false' && rch daemon start &>/dev/null &
fi
```

---

## Monitoring

### Check Worker Load

```bash
rch status --workers
```

Shows per-worker slot usage, health, latency.

### Check Active Jobs

```bash
rch status --jobs
```

Shows what's being compiled right now.

### Aggregate Stats

```bash
rch status --stats
```

Shows total builds offloaded, time saved, etc.

---

## Emergency Procedures

### RCH Daemon Crashed

```bash
# Quick restart
pkill -9 rchd; sleep 1; rch daemon start

# Verify
rch daemon status --json
```

### All Workers Down

Fall back to local builds (RCH automatically does this):

```bash
# Temporarily disable RCH
export RCH_BYPASS=1
cargo build

# Or reduce agent count to avoid CPU storms
ntm send project --all "/exit"
# Then spawn fewer agents
ntm spawn project --cc=3
```

### Hook Not Working

```bash
# Check hook is installed
grep -A5 "PreToolUse" ~/.claude/settings.json

# Reinstall
rch hook install
```

---

## Expected Impact

With RCH properly configured:

| Scenario | Without RCH | With RCH |
|----------|-------------|----------|
| 5 agents, each builds | Load 5x+ | Load ~1x |
| 15 agents, concurrent builds | System freeze | Load ~1.5x |
| Full rebuild, 10 agents | 30+ min, unresponsive | 5 min, responsive |

**The bottom line:** If you're running more than 3 agents and doing Rust development, RCH is essential.
