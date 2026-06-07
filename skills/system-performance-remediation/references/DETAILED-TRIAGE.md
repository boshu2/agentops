# Detailed Process Triage

Step-by-step triage procedures for each category of killable processes.

---

## Step 1: Zombies (Always Safe)

Zombies consume no resources except a slot in the process table. They cannot be killed — only their parent can reap them.

```bash
# Find zombies and their parents
ps aux | awk '$8=="Z" {print $2, $11}' | while read zpid zname; do
  ppid=$(ps -o ppid= -p $zpid 2>/dev/null | tr -d ' ')
  pname=$(ps -o comm= -p $ppid 2>/dev/null)
  echo "Zombie: $zpid ($zname) <- Parent: $ppid ($pname)"
done

# Trigger reaping by signaling parent
kill -SIGCHLD $PARENT_PID  # Polite request to reap

# If parent is <defunct> or unresponsive, parent must die first
```

---

## Step 2: Stuck Tests (Very Safe)

Identify by: running 12+ hours, command contains `test`, low recent CPU.

```bash
# Find stuck test processes
ps -eo pid,etimes,pcpu,args --sort=-etimes | \
  grep -E 'bun test|cargo test|jest|pytest|vitest' | \
  awk '$2 > 43200 {print $1, $3"%", int($2/3600)"h", $4}'

# Kill stuck tests (SIGTERM first, SIGKILL if needed)
for pid in $(ps -eo pid,etimes,args | grep 'bun test' | awk '$2 > 43200 {print $1}'); do
  echo "Killing stuck test: $pid"
  kill $pid
  sleep 2
  kill -0 $pid 2>/dev/null && kill -9 $pid
done
```

---

## Step 3: Abandoned Dev Servers

Identify by: running 24+ hours, near-zero CPU, port-binding servers.

```bash
# Find old dev servers
ps -eo pid,etimes,pcpu,args --sort=-etimes | \
  grep -E 'next dev|bun --hot|vite|webpack serve|npm run dev' | \
  awk '$2 > 86400 && $3 < 1 {print $1, int($2/3600)"h", $3"%", $4}'

# Kill with SIGTERM (they should handle graceful shutdown)
kill $PID
```

---

## Step 4: Stale NTM/Tmux Sessions

```bash
# List all tmux sessions with age
/usr/bin/tmux list-sessions -F '#{session_name} #{session_created}' 2>/dev/null | while read name created; do
  age_hours=$(( ($(date +%s) - created) / 3600 ))
  echo "$name: ${age_hours}h old"
done

# Kill specific stale sessions (check first!)
/usr/bin/tmux kill-session -t "ntm-old-session"

# Nuclear option: kill ALL ntm sessions (use with caution)
/usr/bin/tmux list-sessions -F '#{session_name}' | grep '^ntm-' | xargs -I{} /usr/bin/tmux kill-session -t {}
```

### Session Naming Convention

| Pattern | Purpose | Safe to Kill? |
|---------|---------|---------------|
| `ntm-test-*` | Test artifacts | Yes |
| `ntm-lifecycle-*` | Lifecycle tests | Yes |
| `ntm-rapid-*` | Rapid spawn tests | Yes |
| `projectname` | Actual work session | Check first! |

---

## Step 5: Stuck Vercel/Git Commands

Vercel CLI and git operations can hang indefinitely, consuming massive CPU.

```bash
# Find stuck vercel commands (5+ minutes is suspicious)
ps -eo pid,etimes,pcpu,args | grep -E 'vercel|git add' | grep -v grep | \
  awk '$2 > 300 {print $1, int($2/60)"m", $3"%", substr($0, index($0,$4))}'

# Kill stuck vercel inspect/env commands
ps -eo pid,etimes,args | grep 'vercel' | awk '$2 > 600 {print $1}' | xargs -r kill

# Kill stuck git add (should never take more than a few seconds)
ps -eo pid,etimes,args | grep 'git add' | awk '$2 > 120 {print $1}' | xargs -r kill -9
```

---

## Step 6: Old Agent Sessions

**CAUTION:** Agents may be doing useful work. Check activity first.

```bash
# Find old Claude/Codex sessions (24+ hours)
ps -eo pid,etimes,pcpu,args | grep -E 'claude|codex' | awk '$2 > 86400'

# Check if they're actively working
ps -eo pid,pcpu,etimes,args | grep claude | awk '{if($2>5) print "ACTIVE:", $0; else if($3>86400) print "MAYBE_STALE:", $0}'

# Check terminal attachment
for pid in $(pgrep -f 'claude|codex'); do
  tty=$(ps -o tty= -p $pid)
  if [[ "$tty" == "?" ]]; then
    echo "$pid: detached (possibly abandoned)"
  fi
done
```

---

## Identity Validation

Before killing, verify process identity to prevent PID reuse attacks:

```bash
# Get boot ID + start time + PID = unique identity
boot_id=$(cat /proc/sys/kernel/random/boot_id)
start_time=$(stat -c %Y /proc/$PID 2>/dev/null)
identity="${boot_id}:${start_time}:${PID}"
```

---

## Competing Builds Detection

**The #1 cause of agent swarm meltdowns.** Multiple agents building the same project simultaneously with different target directories.

### Detect Competing Builds

```bash
# Check for multiple duckdb builds (the usual culprit - takes forever to compile)
echo '=== DUCKDB BUILD LOCATIONS ===' && \
ps aux | grep -E 'cc1plus|libduckdb-sys' | grep -v grep | \
  grep -oP 'target[^/]*/' | sort | uniq -c

# If you see multiple targets, you have competing builds:
#    4279 target-maroon/
#    3877 target-whitecastle/   <-- BAD! Two agents fighting

# Find ALL unique CARGO_TARGET_DIR values
echo '=== ALL CARGO TARGET DIRS ===' && \
for pid in $(ps -eo pid,args | grep '/bin/cargo' | grep -v grep | awk '{print $1}'); do
  dir=$(cat /proc/$pid/environ 2>/dev/null | tr '\0' '\n' | grep CARGO_TARGET_DIR | cut -d= -f2)
  age=$(ps -o etimes= -p $pid)
  cmd=$(ps -o args= -p $pid | head -c 50)
  echo "${age}s | $dir | $cmd"
done | sort -n
```

### Kill Duplicate Cargo Builds

```bash
# Find duplicate cargo check --all-targets (keep only newest)
ps -eo pid,etimes,args | grep 'cargo check --all-targets' | grep -v grep | \
  sort -k2 -rn | tail -n +2 | awk '{print $1}' | xargs -r kill

# Or kill by specific target dir
ps -eo pid,args | grep '/bin/cargo' | while read pid rest; do
  dir=$(cat /proc/$pid/environ 2>/dev/null | tr '\0' '\n' | grep CARGO_TARGET_DIR | cut -d= -f2)
  [[ "$dir" == "target-whitecastle" ]] && kill $pid && echo "Killed $pid ($dir)"
done
```

### Find Parent Agent

```bash
# Find which agent spawned the cargo process
CARGO_PID=489714
AGENT_PID=$(ps -o ppid= -p $CARGO_PID | tr -d ' ')
ps -o args= -p $AGENT_PID  # Shows: claude --dangerously-skip-permissions

# The agent is the problem, not the cargo build
```

---

## Confused Agents in Loops

Agents running 16+ hours are often stuck in unproductive loops, repeatedly triggering the same builds or commands.

### Identify Confused Agents

```bash
# List all agents by age
echo '=== AGENT SESSIONS BY AGE ===' && \
ps -eo pid,etimes,pcpu,args | grep -E 'claude --dangerously|codex --dangerously' | \
  grep -v grep | awk '{print $1, int($2/3600)"h", $3"%"}' | sort -t'h' -k2 -rn

# Find very old agents (16+ hours) - likely stuck
echo '=== OLD AGENTS (16+ hours) ===' && \
ps -eo pid,etimes,pcpu,args | grep -E 'claude --dangerously|codex --dangerously' | \
  grep -v grep | awk '$2 > 57600 {print $1, int($2/3600)"h", $3"%", substr($0, index($0,$4), 60)}'
```

### Check What Old Agents Are Doing

```bash
# See children of old agents - if only MCP servers, they may be idle/stuck
for pid in $(ps -eo pid,etimes,args | grep -E 'claude|codex' | awk '$2 > 57600 {print $1}'); do
  echo "PID $pid children:"
  pgrep -P $pid 2>/dev/null | head -3 | while read cpid; do
    ps -o args= -p $cpid 2>/dev/null | head -c 80
    echo
  done
  echo
done

# If children are only "npm exec @morphllm/morphmcp" - agent is likely idle/stuck
# If children include active cargo/test commands - agent may still be working
```

### Kill Confused Old Agents

```bash
# Kill agents older than 16 hours
echo '=== KILLING OLD AGENTS (16+ hours) ===' && \
ps -eo pid,etimes,pcpu,args | grep -E 'claude --dangerously|codex --dangerously' | \
  grep -v grep | awk '$2 > 57600 {print $1, int($2/3600)"h", $3"%"}' | \
  while read pid age cpu; do
    echo "Killing $pid ($age, $cpu CPU)"
    kill $pid 2>/dev/null
  done

# Verify reduction
sleep 5 && echo "Remaining agents:" && \
echo -n "  claude: " && pgrep -f 'claude --dangerously' | wc -l && \
echo -n "  codex: " && pgrep -f 'codex --dangerously' | wc -l
```

---

## Renice Instead of Kill

When you can't kill processes (legitimate work) but need responsiveness, **renice them**.

```bash
# Set CPU priority to lowest (nice 19)
echo '=== RENICING COMPILATION PROCESSES ===' && \
for pid in $(pgrep -f '/bin/cargo') $(pgrep cc1plus) $(pgrep rustc); do
  renice 19 -p $pid 2>/dev/null
done && echo "Reniced all cargo/cc1plus/rustc to nice 19"

# Set IO priority to idle class (only uses IO when nothing else needs it)
echo '=== SETTING IONICE TO IDLE ===' && \
for pid in $(pgrep -f '/bin/cargo') $(pgrep cc1plus) $(pgrep rustc); do
  ionice -c 3 -p $pid 2>/dev/null
done && echo "Set all compilation to ionice class 3 (idle)"
```

**Note:** Renicing doesn't reduce load average - it just ensures interactive processes get priority. Load will still look high, but the system will feel more responsive.

---

## Before/After Verification

Always measure impact:

```bash
# Capture before state
echo "BEFORE: Load=$(uptime | awk -F'load average:' '{print $2}') Mem=$(free -h | awk '/Mem:/{print $7}')"

# [Do cleanup actions]

# Wait for kernel to settle
sleep 10

# Capture after state
echo "AFTER:  Load=$(uptime | awk -F'load average:' '{print $2}') Mem=$(free -h | awk '/Mem:/{print $7}')"
```
