# Agent Swarm Performance Patterns

Common patterns and anti-patterns when running multi-agent swarms.

## The Competing Builds Problem

### Symptom
- Load average 3-5x core count (e.g., 200+ on 64-core machine)
- Multiple cc1plus/rustc processes with different target directories
- System feels sluggish despite "killing stuck processes"
- Same processes keep respawning after being killed

### Root Cause
Multiple agents independently building the same project. Each agent uses a different `CARGO_TARGET_DIR` for isolation, but this means:
- No shared compilation cache
- Disk I/O thrashing between builds
- CPU contention multiplied by number of agents

### Detection

```bash
# Count builds by target directory
ps aux | grep cc1plus | grep -oP 'target[^/]*/' | sort | uniq -c

# Bad output (competing):
#    4279 target-maroon/
#    3877 target-whitecastle/
#    2100 target-olivepeak/

# Good output (single build):
#    4279 target-maroon/
```

### Solution
1. **Don't kill the builds** — agents will respawn them
2. **Kill the confused old agents** (16+ hours) spawning duplicates
3. **Renice remaining builds** to prevent future sluggishness

## The Whack-a-Mole Anti-Pattern

### Symptom
You kill a process, it comes back. Kill again, comes back again.

### Why It Happens
The process you're killing is a **child** of an agent. The agent's control loop detects the child died and restarts it.

### Detection
```bash
# Find the parent of the respawning process
CHILD_PID=489714
PARENT_PID=$(ps -o ppid= -p $CHILD_PID | tr -d ' ')
ps -o pid,etimes,args -p $PARENT_PID
# Output: claude --dangerously-skip-permissions (age: 64800s = 18 hours)
```

### Solution
Kill the parent agent, not the child process.

## Agent Age Heuristics

| Age | CPU% | Status | Action |
|-----|------|--------|--------|
| <4h | >20% | Actively working | Protect |
| <4h | <5% | Waiting/thinking | Protect |
| 4-16h | >20% | Probably working | Check children |
| 4-16h | <5% | Possibly stuck | Check children |
| 16+h | any | Likely stuck in loop | Kill |

### Checking Agent Health

```bash
# What are the agent's children doing?
for pid in 5521 902432; do
  echo "PID $pid children:"
  pgrep -P $pid | while read cpid; do
    ps -o args= -p $cpid | head -c 80
  done
done

# Good children (active work):
# /home/ubuntu/.rustup/.../cargo check --all-targets
# node /home/ubuntu/.bun/bin/vercel deploy

# Bad children (idle MCP servers only):
# npm exec @morphllm/morphmcp
# (no other children = agent is stuck/idle)
```

## CPU Pressure vs Load Average

Load average can be misleading. Use CPU pressure for accurate diagnosis.

### Load Average Limitations
- Includes processes in uninterruptible sleep (D state)
- Can spike temporarily during normal operation
- Doesn't distinguish between "waiting for CPU" vs "waiting for I/O"

### CPU Pressure Advantage
```bash
cat /proc/pressure/cpu
# some avg10=57.18 avg60=58.76 avg300=54.02

# avg10 > 30% = tasks are waiting for CPU = sluggish system
# avg10 < 10% = healthy, even if load average looks high
```

| Load Avg | CPU Pressure | Reality |
|----------|--------------|---------|
| High | Low | I/O bound, not CPU bound |
| High | High | CPU contention, kill competing work |
| Low | High | Unusual, check for throttling |
| Low | Low | System healthy |

## Renice Strategy

When you can't kill (legitimate work), make it yield to interactive tasks.

```bash
# CPU priority: nice 19 = lowest
renice 19 -p $PID

# I/O priority: class 3 = idle (only uses I/O when system idle)
ionice -c 3 -p $PID
```

**Important:** Renicing doesn't reduce load average. The processes still count as "runnable." But interactive processes will get priority, so the system feels responsive.

## Typical Remediation Sequence

1. **Diagnose**: `uptime`, `/proc/pressure/cpu`, competing builds check
2. **Kill stuck tests/commands**: Safe, immediate relief
3. **Kill duplicate builds**: Keep only newest per target
4. **Identify confused agents**: 16+ hours, only MCP children
5. **Kill confused agents**: Root cause fix
6. **Renice remaining builds**: Prevent future sluggishness
7. **Monitor**: Watch load and CPU pressure drop

## Prevention

- Limit concurrent agents per project
- Use shared `CARGO_TARGET_DIR` when possible (requires locking)
- Set up agent coordination (see `agent-mail` skill)
- Configure agents to check for existing builds before starting new ones

---

## Advanced Monitoring Tools

### pidstat (Best for Process-Level Monitoring)

```bash
# CPU by process with 1-second intervals
pidstat -u 1 3 | grep -E 'PID|claude|cargo|cc1plus'

# IO by process (find disk thrashers)
pidstat -d 1 3 | head -30

# Combined CPU + IO + Memory for specific PID
pidstat -urd -p $PID 1 5
```

### htop Batch Mode

```bash
# Non-interactive, sort by CPU
htop -d 1 -n 1 --sort-key=PERCENT_CPU 2>/dev/null | head -30

# Filter to cargo processes
htop -F "cargo" -d 1 -n 1 2>/dev/null
```

### Why pidstat > ps for Monitoring

| Tool | Best For |
|------|----------|
| `ps` | Point-in-time snapshots, finding processes |
| `pidstat` | Ongoing monitoring, CPU/IO correlation |
| `htop` | Interactive exploration |
| `/proc/pressure/*` | System-wide pressure metrics |

---

## macOS vs Linux Differences

| Task | Linux | macOS |
|------|-------|-------|
| Memory pressure | `cat /proc/pressure/memory` | `memory_pressure` |
| Drop caches | `sysctl -w vm.drop_caches=3` | Not available |
| Process list | `ps aux --sort=-%cpu` | `top -l 1 -o cpu` |
| CPU count | `nproc` | `sysctl -n hw.ncpu` |
| Free memory | `free -h` | `vm_stat` (pages, not bytes) |

### macOS Memory Calculation

```bash
# macOS vm_stat gives pages, convert to GB:
vm_stat | awk '/Pages free/ {free=$3} /page size/ {ps=$8} END {print free*ps/1024/1024/1024 " GB free"}'
```
