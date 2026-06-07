# Detailed System Diagnosis

Comprehensive diagnostic commands and techniques.

---

## Full System Status (Linux)

```bash
# 1. Load average vs CPU count
uptime && nproc
# Danger: load_avg / nproc > 1.5 = overloaded

# 2. Memory pressure
free -h
# Danger: available < 10% of total

# 3. Zombie count
ps -eo stat | grep -c '^Z' || echo 0

# 4. IO pressure (if available)
cat /proc/pressure/io 2>/dev/null || iostat -x 1 1

# 5. File handle usage
cat /proc/sys/fs/file-nr  # (allocated, free, max)

# 6. Top consumers
ps aux --sort=-%cpu | head -20
ps aux --sort=-%mem | head -10
```

---

## Full System Status (macOS)

```bash
# 1. Load average and top processes
top -l 1 -n 10 -stats pid,command,cpu,mem,state | head -20

# 2. Memory pressure (macOS-specific)
memory_pressure
# "System-wide memory free percentage: X%"
# < 20% = warning, < 10% = critical

# 3. Virtual memory stats
vm_stat | head -15

# 4. Disk usage
df -h /

# 5. Total RAM
sysctl hw.memsize | awk '{print $2/1024/1024/1024 " GB"}'
```

### macOS Memory Calculation

```bash
# macOS vm_stat gives pages, convert to GB:
vm_stat | awk '/Pages free/ {free=$3} /page size/ {ps=$8} END {print free*ps/1024/1024/1024 " GB free"}'
```

---

## Advanced Monitoring with pidstat

**pidstat is more useful than ps for ongoing monitoring:**

```bash
# CPU usage by process (1 second intervals, 3 samples)
pidstat -u 1 3 | grep -E 'PID|claude|cargo|cc1plus|bun'

# IO usage by process (see who's thrashing disk)
pidstat -d 1 3 | head -30

# Memory usage by process
pidstat -r 1 3 | head -30

# Combined: CPU + IO + Memory for specific process
pidstat -urd -p $(pgrep -f 'cargo check' | head -1) 1 5
```

---

## htop Batch Mode (Non-Interactive)

```bash
# One-shot top CPU consumers (machine-readable)
htop -d 1 -n 1 --sort-key=PERCENT_CPU 2>/dev/null | head -30

# Filter to specific processes
htop -F "cargo" -d 1 -n 1 2>/dev/null | head -20
```

---

## One-Liner Assessment

```bash
echo "Load: $(uptime | awk -F'load average:' '{print $2}') / $(nproc) cores | Mem: $(free -h | awk '/Mem:/{print $3"/"$2}') | Zombies: $(ps -eo stat | grep -c '^Z' || echo 0) | FDs: $(cat /proc/sys/fs/file-nr | awk '{print $1"/"$3}')"
```

---

## CPU Pressure (Critical for Sluggishness)

**Load average can look "OK" while system feels sluggish. CPU pressure reveals the truth.**

```bash
# CPU pressure - if avg10 > 30%, tasks are waiting for CPU
cat /proc/pressure/cpu
# some avg10=57.18 avg60=58.76 avg300=54.02 total=39231816941
# ^^^^ This means 57% of time, tasks were waiting for CPU!

# IO pressure - rarely the culprit on NVMe
cat /proc/pressure/io

# Memory pressure
cat /proc/pressure/memory
```

### Pressure Thresholds

| Metric | Healthy | Warning | Critical |
|--------|---------|---------|----------|
| CPU pressure avg10 | <10% | 10-30% | >30% |
| IO pressure avg10 | <5% | 5-15% | >15% |
| Memory pressure avg10 | <5% | 5-20% | >20% |

### Memory Pressure: The Silent Sluggishness Killer

Memory pressure is **often the real cause** when a machine feels sluggish but load and CPU pressure
look OK. Unlike CPU pressure, memory pressure comes from filesystem cache bloat (btrfs/ext4 dentry
and inode caches) rather than process memory usage.

```bash
# Memory pressure check with context
cat /proc/pressure/memory
# some avg10=18.78 avg60=16.49 avg300=14.54 → sustained 15% = processes stalling on reclaim

# What's causing it?
sysctl vm.vfs_cache_pressure  # If < 150 on a server → cache bloat
grep -E "Slab|Cached" /proc/meminfo  # Slab > 10% of RAM → cache bloat
sudo slabtop -o -s c | head -10  # btrfs_inode, radix_tree_node, ext4_inode_cache
```

**Root cause pattern:** `vfs_cache_pressure=50` or `100` + high-RAM btrfs/ext4 machine + many
agents reading/writing files = massive dentry/inode cache buildup → kernel reclaim path stalls
→ all processes experience latency → system feels sluggish despite "free" RAM.

### CPU Pressure vs Load Average

| Load Avg | CPU Pressure | Reality |
|----------|--------------|---------|
| High | Low | I/O bound, not CPU bound |
| High | High | CPU contention, kill competing work |
| Low | High | Unusual, check for throttling |
| Low | Low | System healthy |

### Memory Pressure vs Free RAM

| Free RAM | Mem Pressure | Reality |
|----------|-------------|---------|
| Low | High | Actual memory exhaustion — find the hog |
| **High** | **High** | **Cache bloat — fix vfs_cache_pressure + drop caches** |
| Low | Low | Pages are in use but not under reclaim pressure |
| High | Low | System healthy |

---

## Why pidstat > ps for Monitoring

| Tool | Best For |
|------|----------|
| `ps` | Point-in-time snapshots, finding processes |
| `pidstat` | Ongoing monitoring, CPU/IO correlation |
| `htop` | Interactive exploration |
| `/proc/pressure/*` | System-wide pressure metrics |

---

## File Handle Diagnostics

```bash
# System-wide status
cat /proc/sys/fs/file-nr  # allocated, free, max

# Per-process FD counts (top offenders)
for pid in $(ps -e -o pid --no-headers); do
  count=$(ls /proc/$pid/fd 2>/dev/null | wc -l)
  [[ $count -gt 100 ]] && echo "$count FDs: $(ps -o comm= -p $pid) (PID $pid)"
done | sort -rn | head -20

# Specific process
ls -la /proc/$PID/fd | wc -l
```

### Temporary File Handle Relief

```bash
# Increase system limit (temporary, until reboot)
sudo sysctl -w fs.file-max=2097152

# Increase user limit (current session only)
ulimit -n 65536
```

---

## Memory Cache Relief

When memory pressure is high but no single process is the culprit:

### Step 1: Fix VM Tuning (Root Cause)

Dropping caches without fixing VM tuning is treating the symptom. The cache will rebuild.

```bash
# Check current tuning
sysctl vm.vfs_cache_pressure vm.min_free_kbytes

# Fix (btrfs: 200, ext4: 150; adjust min_free per machine size)
sudo sysctl -w vm.vfs_cache_pressure=200 vm.min_free_kbytes=2097152

# Persist to /etc/sysctl.d/99-system-resource-protection.conf
```

### Step 2: Drop Caches (Immediate Relief)

```bash
# Check current memory state
free -h
cat /proc/pressure/memory

# Drop page cache, dentries, and inodes (safe, no data loss)
sudo sh -c "sync; echo 3 > /proc/sys/vm/drop_caches"

# Verify improvement (wait 30s for pressure metrics to update)
sleep 30 && free -h && cat /proc/pressure/memory
```

**When to use:** Memory pressure avg10 > 5%, but `available` is low even though no process is hogging memory. The kernel is caching aggressively.

**Safe?** Yes - this only drops clean cached data. Dirty pages are synced first.

**NOTE:** On large-RAM machines (499GB), dropping caches can take 30-60 seconds as the kernel
frees hundreds of GB of page cache. The SSH command may appear to hang — give it time.

### Memory Pressure Relief (macOS)

```bash
# Check memory pressure
memory_pressure

# macOS doesn't have drop_caches equivalent
# Instead, identify and quit memory hogs:
top -l 1 -o mem -n 10

# Force garbage collection in specific apps (if they support it)
# Or simply restart the memory-heavy processes
```

---

## macOS vs Linux Differences

| Task | Linux | macOS |
|------|-------|-------|
| Memory pressure | `cat /proc/pressure/memory` | `memory_pressure` |
| Drop caches | `sysctl -w vm.drop_caches=3` | Not available |
| Process list | `ps aux --sort=-%cpu` | `top -l 1 -o cpu` |
| CPU count | `nproc` | `sysctl -n hw.ncpu` |
| Free memory | `free -h` | `vm_stat` (pages, not bytes) |
