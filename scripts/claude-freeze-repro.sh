#!/usr/bin/env bash
set -euo pipefail
# claude-freeze-repro.sh — Capture diagnostic evidence of Claude Code
# CPU freeze triggered by TaskStop (or any tool interruption).
#
# PROBLEM:  Once the freeze hits, the system can't fork new processes.
# SOLUTION: Start all monitors BEFORE the trigger so they're already
#           capturing when the event loop wedges.
#
# Usage:
#   Terminal A: Run a Claude Code session normally
#   Terminal B: ./scripts/claude-freeze-repro.sh [duration_seconds]
#               (monitors start, waits for you to trigger)
#   Terminal A: In Claude, run a background task then stop it
#   Terminal B: Press ENTER after freeze (or let it auto-detect)
#
# Output: /tmp/claude-freeze-diag-<timestamp>/
#         Attach the whole directory to the GitHub issue.

DURATION="${1:-180}"  # seconds to monitor (default 3 min)
POLL_INTERVAL=1       # seconds between snapshots
DIAG_DIR="/tmp/claude-freeze-diag-$(date +%Y%m%dT%H%M%S)"
mkdir -p "$DIAG_DIR"

cleanup_pids=()
cleanup() {
    for pid in "${cleanup_pids[@]}"; do
        kill "$pid" 2>/dev/null || true
    done
    wait 2>/dev/null || true
}
trap cleanup EXIT

log() { printf '[%s] %s\n' "$(date -u +%H:%M:%S)" "$*"; }

# ── Step 1: Find Claude Code process ────────────────────────────
log "Searching for Claude Code process..."

# macOS pgrep doesn't reliably match Bun/Node binaries that override argv[0].
# Use ps + grep instead: find processes whose comm is exactly "claude".
CLAUDE_PID=""
CLAUDE_CPU="0"
BEST_CPU_INT=0

while IFS= read -r line; do
    pid=$(echo "$line" | awk '{print $1}')
    cpu=$(echo "$line" | awk '{print $2}')
    cpu_int=${cpu%%.*}
    if [[ "${cpu_int:-0}" -gt "$BEST_CPU_INT" ]] || [[ -z "$CLAUDE_PID" ]]; then
        CLAUDE_PID="$pid"
        CLAUDE_CPU="$cpu"
        BEST_CPU_INT="${cpu_int:-0}"
    fi
done < <(ps -eo pid=,pcpu=,comm=,args= 2>/dev/null | awk '$3 == "claude" && $4 == "claude"')

if [[ -z "$CLAUDE_PID" ]]; then
    echo "ERROR: No Claude Code process found."
    echo "Start a Claude Code session in another terminal first."
    echo ""
    echo "Processes with 'claude' in name:"
    ps -eo pid,pcpu,comm 2>/dev/null | grep -i claude || echo "  (none)"
    exit 1
fi

log "Found PID $CLAUDE_PID (claude) at ${CLAUDE_CPU}% CPU"

# ── Step 2: Capture environment baseline ────────────────────────
log "Capturing baseline..."

{
    echo "=== Environment ==="
    echo "Date:     $(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "Hostname: $(hostname)"
    echo "OS:       $(sw_vers -productName 2>/dev/null || uname -s) $(sw_vers -productVersion 2>/dev/null || uname -r)"
    echo "Kernel:   $(uname -r)"
    echo "Arch:     $(uname -m)"
    echo "CPUs:     $(sysctl -n hw.ncpu 2>/dev/null || nproc 2>/dev/null || echo unknown)"
    echo "RAM:      $(sysctl -n hw.memsize 2>/dev/null | awk '{printf "%.0f GB", $1/1073741824}' 2>/dev/null || echo unknown)"
    echo ""
    echo "=== Claude Code ==="
    echo "PID:      $CLAUDE_PID"
    timeout 5 claude --version 2>/dev/null || echo "claude --version: not available"
    echo ""
    echo "=== Claude processes (sanitized — comm only, no args) ==="
    ps -eo pid,ppid,stat,pcpu,pmem,rss,comm 2>/dev/null | head -1 || true
    ps -eo pid,ppid,stat,pcpu,pmem,rss,comm 2>/dev/null | grep -i claude | grep -v grep || true
    echo ""
    echo "=== Open FDs (baseline) ==="
    timeout 10 lsof -p "$CLAUDE_PID" 2>/dev/null | head -50 || true
} > "$DIAG_DIR/00-baseline.txt" 2>&1

log "Baseline saved"

# ── Step 3: Start monitors ──────────────────────────────────────
log "Starting monitors (${DURATION}s window)..."

# Monitor A: macOS CPU profiler — samples call stacks
# This is the money shot: shows exactly what function is spinning
if command -v sample &>/dev/null; then
    log "  [A] sample (call-stack profiler, ${DURATION}s)"
    sample "$CLAUDE_PID" "$DURATION" -f "$DIAG_DIR/01-sample.txt" 2>/dev/null &
    cleanup_pids+=($!)
fi

# Monitor B: spindump — macOS hang detector (needs root)
if command -v spindump &>/dev/null && [[ $(id -u) -eq 0 ]]; then
    log "  [B] spindump (hang detector, ${DURATION}s)"
    spindump "$CLAUDE_PID" "$DURATION" -file "$DIAG_DIR/02-spindump.txt" 2>/dev/null &
    cleanup_pids+=($!)
else
    log "  [B] spindump skipped (needs: sudo $0)"
fi

# Monitor C: CPU% timeline — 1 sample/sec via ps polling
log "  [C] CPU timeline (ps polling, ${POLL_INTERVAL}s interval)"
(
    echo "timestamp,pid,pcpu,pmem,rss_kb,vsz_kb,state,threads"
    end=$((SECONDS + DURATION))
    while [[ $SECONDS -lt $end ]]; do
        ts=$(date -u +%Y-%m-%dT%H:%M:%SZ)
        ps -p "$CLAUDE_PID" -o pid=,pcpu=,pmem=,rss=,vsz=,state= 2>/dev/null | while read -r pid pcpu pmem rss vsz state; do
            nlwp=$(ps -M -p "$CLAUDE_PID" 2>/dev/null | tail -n +2 | wc -l | tr -d ' ')
            echo "${ts},${pid},${pcpu},${pmem},${rss},${vsz},${state},${nlwp}"
        done
        sleep "$POLL_INTERVAL"
    done
) > "$DIAG_DIR/03-cpu-timeline.csv" 2>&1 &
cleanup_pids+=($!)

# Monitor D: Child process tracker — catches zombie/orphan children
log "  [D] Child process tracker (2s interval)"
(
    end=$((SECONDS + DURATION))
    while [[ $SECONDS -lt $end ]]; do
        echo "--- $(date -u +%Y-%m-%dT%H:%M:%SZ) ---"
        pgrep -P "$CLAUDE_PID" 2>/dev/null | while read -r child; do
            ps -p "$child" -o pid,ppid,stat,pcpu,rss,comm 2>/dev/null || true
            pgrep -P "$child" 2>/dev/null | while read -r grandchild; do
                echo "  └─ $(ps -p "$grandchild" -o pid,ppid,stat,pcpu,rss,comm 2>/dev/null || true)"
            done
        done
        echo ""
        sleep 2
    done
) > "$DIAG_DIR/04-children.txt" 2>&1 &
cleanup_pids+=($!)

# Monitor E: Open file descriptors over time (pipes are the likely culprit)
log "  [E] FD/pipe tracker (5s interval)"
(
    end=$((SECONDS + DURATION))
    while [[ $SECONDS -lt $end ]]; do
        echo "--- $(date -u +%Y-%m-%dT%H:%M:%SZ) ---"
        timeout 5 lsof -p "$CLAUDE_PID" 2>/dev/null | grep -E 'PIPE|FIFO|pipe|CHR' || echo "(no pipes)"
        echo ""
        sleep 5
    done
) > "$DIAG_DIR/05-fd-pipes.txt" 2>&1 &
cleanup_pids+=($!)

# Monitor F: System load (proves the freeze starves other processes)
log "  [F] System load tracker (${POLL_INTERVAL}s interval)"
(
    end=$((SECONDS + DURATION))
    while [[ $SECONDS -lt $end ]]; do
        ts=$(date -u +%Y-%m-%dT%H:%M:%SZ)
        load=$(sysctl -n vm.loadavg 2>/dev/null | awk '{print $2, $3, $4}' || uptime | awk -F'load average:' '{print $2}')
        echo "${ts} load: ${load}"
        sleep "$POLL_INTERVAL"
    done
) > "$DIAG_DIR/06-load.txt" 2>&1 &
cleanup_pids+=($!)

# Brief pause to let monitors initialize
sleep 1

# ── Step 4: Wait for trigger ────────────────────────────────────
cat <<'BANNER'

╔══════════════════════════════════════════════════════════════╗
║  MONITORS RUNNING                                            ║
║                                                              ║
║  Now trigger the freeze in your Claude Code session:         ║
║                                                              ║
║  1. Ask Claude to run a background Bash task                 ║
║     e.g. "run sleep 60 in background"                        ║
║                                                              ║
║  2. Then ask Claude to stop that task                        ║
║     (or it may call TaskStop automatically)                  ║
║                                                              ║
║  Press ENTER after the freeze happens.                       ║
║  (Auto-detects if CPU exceeds 95% for 10s)                  ║
╚══════════════════════════════════════════════════════════════╝

BANNER

# Auto-detect freeze: poll CPU% and trigger capture if sustained spike
(
    spike_count=0
    while true; do
        cpu=$(ps -p "$CLAUDE_PID" -o pcpu= 2>/dev/null | tr -d ' ' || echo "0")
        cpu_int=${cpu%%.*}
        if [[ "${cpu_int:-0}" -ge 95 ]]; then
            spike_count=$((spike_count + 1))
            if [[ "$spike_count" -ge 10 ]]; then
                echo "FREEZE_DETECTED" > "$DIAG_DIR/.freeze-flag"
                log "AUTO-DETECTED: CPU at ${cpu}% for ${spike_count}s"
                {
                    echo "=== Freeze detected at $(date -u +%Y-%m-%dT%H:%M:%SZ) ==="
                    echo "CPU: ${cpu}%"
                    echo "Sustained spike: ${spike_count}s"
                    echo ""
                    echo "=== Process state ==="
                    ps -eo pid,ppid,stat,pcpu,pmem,rss,comm 2>/dev/null | grep -i claude | grep -v grep || true
                    echo ""
                    echo "=== Thread states ==="
                    ps -M -p "$CLAUDE_PID" 2>/dev/null || true
                    echo ""
                    echo "=== Open FDs ==="
                    timeout 10 lsof -p "$CLAUDE_PID" 2>/dev/null | head -100 || true
                } > "$DIAG_DIR/07-freeze-snapshot.txt" 2>&1
                break
            fi
        else
            spike_count=0
        fi
        sleep 1
    done
) &
DETECT_PID=$!
cleanup_pids+=($DETECT_PID)

# Wait for user ENTER or auto-detection
while true; do
    if read -t 2; then
        break
    fi
    if [[ -f "$DIAG_DIR/.freeze-flag" ]]; then
        echo ""
        log "Freeze auto-detected! Continuing capture for 30 more seconds..."
        sleep 30
        break
    fi
done

# ── Step 5: Final capture ───────────────────────────────────────
log "Capturing final state..."

{
    echo "=== Final process state ==="
    echo "Timestamp: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo ""
    ps -eo pid,ppid,stat,pcpu,pmem,rss,comm 2>/dev/null | grep -i claude | grep -v grep || echo "(no claude processes)"
    echo ""
    echo "=== Thread dump ==="
    ps -M -p "$CLAUDE_PID" 2>/dev/null || echo "(process gone)"
    echo ""
    echo "=== Final FDs ==="
    timeout 10 lsof -p "$CLAUDE_PID" 2>/dev/null | head -100 || echo "(process gone)"
    echo ""
    echo "=== System load ==="
    uptime
} > "$DIAG_DIR/08-final-state.txt" 2>&1

# ── Step 6: Stop monitors and package ───────────────────────────
log "Stopping monitors..."
cleanup
trap - EXIT

rm -f "$DIAG_DIR/.freeze-flag"

# Generate summary
cat > "$DIAG_DIR/README.md" <<EOF
# Claude Code TaskStop Freeze — Diagnostic Bundle

**Generated:** $(date -u +%Y-%m-%dT%H:%M:%SZ)
**PID:** $CLAUDE_PID
**Duration:** ${DURATION}s capture window

## Files

| File | Purpose |
|------|---------|
| 00-baseline.txt | Environment + process state before trigger |
| 01-sample.txt | macOS CPU profiler call stacks (**KEY EVIDENCE**) |
| 02-spindump.txt | macOS hang detector output (if run as root) |
| 03-cpu-timeline.csv | CPU% sampled every ${POLL_INTERVAL}s (shows spike onset) |
| 04-children.txt | Child process lifecycle (zombie/orphan detection) |
| 05-fd-pipes.txt | Pipe/FIFO file descriptors (unclosed pipe = likely cause) |
| 06-load.txt | System load average (proves system-wide starvation) |
| 07-freeze-snapshot.txt | Snapshot captured at moment of detected freeze |
| 08-final-state.txt | Process state after capture window |

## How to read the evidence

1. **01-sample.txt** — Look for a single function consuming >90% of samples.
   This is the infinite loop. The function name/address identifies the bug.

2. **03-cpu-timeline.csv** — Import into a spreadsheet. The CPU% column
   should show a sharp jump from normal (<30%) to 100% at the freeze moment.

3. **05-fd-pipes.txt** — Compare before and after freeze. If a PIPE fd
   appears before and persists after the child is killed, that's the
   unclosed pipe causing the event loop to spin.

4. **04-children.txt** — If the stopped background task's process is still
   alive (zombie state Z) after TaskStop, cleanup failed.
EOF

echo ""
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║  CAPTURE COMPLETE                                            ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""
echo "Output: $DIAG_DIR"
echo ""
ls -lh "$DIAG_DIR"
echo ""
echo "To attach to GitHub issue:"
echo "  tar czf /tmp/claude-freeze-diag.tar.gz -C /tmp $(basename "$DIAG_DIR")"
echo ""
echo "Key file to check first: $DIAG_DIR/01-sample.txt"
