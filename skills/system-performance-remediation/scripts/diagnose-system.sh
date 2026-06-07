#!/usr/bin/env bash
# System Performance Diagnostic Script
# Run: ./diagnose-system.sh [--json]

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

JSON_MODE=false
[[ "${1:-}" == "--json" ]] && JSON_MODE=true

pass() { $JSON_MODE || echo -e "${GREEN}OK${NC} $1"; }
warn() { $JSON_MODE || echo -e "${YELLOW}WARN${NC} $1"; }
fail() { $JSON_MODE || echo -e "${RED}CRIT${NC} $1"; }
info() { $JSON_MODE || echo -e "${BLUE}INFO${NC} $1"; }
float_gt() { awk -v lhs="$1" -v rhs="$2" 'BEGIN { exit !(lhs > rhs) }'; }
human_bytes() {
    if command -v numfmt >/dev/null 2>&1; then
        numfmt --to=iec "$1"
    else
        printf '%s' "$1"
    fi
}

# Gather metrics
LOAD_1=$(awk '{print $1}' /proc/loadavg)
LOAD_5=$(awk '{print $2}' /proc/loadavg)
LOAD_15=$(awk '{print $3}' /proc/loadavg)
NPROC=$(nproc)
LOAD_RATIO=$(awk "BEGIN {printf \"%.2f\", $LOAD_1 / $NPROC}")

MEM_TOTAL=$(free -b | awk '/Mem:/{print $2}')
MEM_AVAIL=$(free -b | awk '/Mem:/{print $7}')
MEM_USED_PCT=$(awk "BEGIN {printf \"%.1f\", 100 - ($MEM_AVAIL / $MEM_TOTAL * 100)}")

ZOMBIE_COUNT=$(ps -eo stat= 2>/dev/null | awk '/^Z/{count++} END {print count + 0}')

FD_ALLOCATED=$(awk '{print $1}' /proc/sys/fs/file-nr)
FD_MAX=$(awk '{print $3}' /proc/sys/fs/file-nr)
FD_PCT=$(awk "BEGIN {printf \"%.1f\", $FD_ALLOCATED / $FD_MAX * 100}")

# Check IO pressure if available
IO_PRESSURE="N/A"
if [[ -f /proc/pressure/io ]]; then
    IO_PRESSURE=$(awk '/full/{print $2}' /proc/pressure/io | cut -d= -f2)
fi

# Count process types (pgrep -c returns 1 if no match, so we handle carefully)
CLAUDE_COUNT=$(pgrep -c -f 'node.*claude' 2>/dev/null) || CLAUDE_COUNT=0
CODEX_COUNT=$(pgrep -c -f 'codex' 2>/dev/null) || CODEX_COUNT=0
GEMINI_COUNT=$(pgrep -c -f 'gemini' 2>/dev/null) || GEMINI_COUNT=0
BUN_TEST_COUNT=$(pgrep -c -f 'bun test' 2>/dev/null) || BUN_TEST_COUNT=0
CARGO_TEST_COUNT=$(pgrep -c -f 'cargo test' 2>/dev/null) || CARGO_TEST_COUNT=0
RUSTC_COUNT=$(pgrep -c -f 'rustc' 2>/dev/null) || RUSTC_COUNT=0

# Count tmux sessions
TMUX_SESSIONS=$(tmux list-sessions 2>/dev/null | wc -l) || TMUX_SESSIONS=0
NTM_SESSIONS=$(tmux list-sessions -F '#{session_name}' 2>/dev/null | grep -c '^ntm-') || NTM_SESSIONS=0

# RCH status
RCH_RUNNING="false"
if command -v rch &>/dev/null; then
    rch daemon status --json 2>/dev/null | grep -q '"running": true' && RCH_RUNNING="true"
fi

# JSON output mode
if $JSON_MODE; then
    cat << EOF
{
  "load": {
    "load_1min": $LOAD_1,
    "load_5min": $LOAD_5,
    "load_15min": $LOAD_15,
    "cpu_count": $NPROC,
    "load_ratio": $LOAD_RATIO,
    "status": "$(awk "BEGIN {if ($LOAD_RATIO > 2) print \"critical\"; else if ($LOAD_RATIO > 1.5) print \"warning\"; else print \"ok\"}")"
  },
  "memory": {
    "total_bytes": $MEM_TOTAL,
    "available_bytes": $MEM_AVAIL,
    "used_percent": $MEM_USED_PCT,
    "status": "$(awk "BEGIN {if ($MEM_USED_PCT > 90) print \"critical\"; else if ($MEM_USED_PCT > 80) print \"warning\"; else print \"ok\"}")"
  },
  "zombies": {
    "count": $ZOMBIE_COUNT,
    "status": "$(if [[ $ZOMBIE_COUNT -gt 10 ]]; then echo "warning"; else echo "ok"; fi)"
  },
  "file_descriptors": {
    "allocated": $FD_ALLOCATED,
    "max": $FD_MAX,
    "percent": $FD_PCT,
    "status": "$(awk "BEGIN {if ($FD_PCT > 80) print \"warning\"; else print \"ok\"}")"
  },
  "io_pressure": "$IO_PRESSURE",
  "processes": {
    "claude": $CLAUDE_COUNT,
    "codex": $CODEX_COUNT,
    "gemini": $GEMINI_COUNT,
    "bun_test": $BUN_TEST_COUNT,
    "cargo_test": $CARGO_TEST_COUNT,
    "rustc": $RUSTC_COUNT
  },
  "tmux": {
    "total_sessions": $TMUX_SESSIONS,
    "ntm_sessions": $NTM_SESSIONS
  },
  "rch": {
    "daemon_running": $RCH_RUNNING
  }
}
EOF
    exit 0
fi

# Human-readable output
echo "================================================================"
echo "       SYSTEM PERFORMANCE DIAGNOSTIC REPORT"
echo "================================================================"
echo

# Load average
echo "1. LOAD AVERAGE"
echo "   ─────────────"
echo "   Load: $LOAD_1 / $LOAD_5 / $LOAD_15 (1/5/15 min)"
echo "   CPUs: $NPROC"
echo "   Ratio: ${LOAD_RATIO}x"
if float_gt "$LOAD_RATIO" 2; then
    fail "CRITICAL: Load ${LOAD_RATIO}x capacity - system overloaded"
elif float_gt "$LOAD_RATIO" 1.5; then
    warn "ELEVATED: Load ${LOAD_RATIO}x capacity"
else
    pass "Load within normal range"
fi
echo

# Memory
echo "2. MEMORY"
echo "   ──────"
echo "   Total: $(human_bytes "$MEM_TOTAL")"
echo "   Available: $(human_bytes "$MEM_AVAIL")"
echo "   Used: ${MEM_USED_PCT}%"
if float_gt "$MEM_USED_PCT" 90; then
    fail "CRITICAL: Memory ${MEM_USED_PCT}% used"
elif float_gt "$MEM_USED_PCT" 80; then
    warn "ELEVATED: Memory ${MEM_USED_PCT}% used"
else
    pass "Memory within normal range"
fi
echo

# Zombies
echo "3. ZOMBIE PROCESSES"
echo "   ─────────────────"
echo "   Count: $ZOMBIE_COUNT"
if [[ $ZOMBIE_COUNT -gt 10 ]]; then
    warn "Elevated zombie count - check parent processes"
    ps aux | awk '$8=="Z" {print "   - PID", $2, $11}'
else
    pass "Zombie count normal"
fi
echo

# File descriptors
echo "4. FILE DESCRIPTORS"
echo "   ─────────────────"
echo "   Allocated: $FD_ALLOCATED / $FD_MAX (${FD_PCT}%)"
if float_gt "$FD_PCT" 80; then
    warn "File descriptor usage elevated"
else
    pass "File descriptors within limits"
fi
echo

# IO pressure
echo "5. IO PRESSURE"
echo "   ───────────"
if [[ "$IO_PRESSURE" != "N/A" ]]; then
    echo "   Full stall: ${IO_PRESSURE}%"
    if float_gt "$IO_PRESSURE" 10; then
        warn "Significant IO pressure detected"
    else
        pass "IO pressure normal"
    fi
else
    info "IO pressure stats not available"
fi
echo

# Process counts
echo "6. PROCESS COUNTS"
echo "   ───────────────"
echo "   Claude agents: $CLAUDE_COUNT"
echo "   Codex agents: $CODEX_COUNT"
echo "   Gemini agents: $GEMINI_COUNT"
echo "   Bun tests: $BUN_TEST_COUNT"
echo "   Cargo tests: $CARGO_TEST_COUNT"
echo "   rustc compilers: $RUSTC_COUNT"

TOTAL_AGENTS=$((CLAUDE_COUNT + CODEX_COUNT + GEMINI_COUNT))
if [[ $TOTAL_AGENTS -gt 20 ]]; then
    warn "High agent count ($TOTAL_AGENTS) - consider reducing swarm size"
fi
if [[ $BUN_TEST_COUNT -gt 5 ]]; then
    warn "Multiple bun test processes - check for stuck tests"
fi
echo

# Tmux sessions
echo "7. TMUX SESSIONS"
echo "   ──────────────"
echo "   Total: $TMUX_SESSIONS"
echo "   NTM: $NTM_SESSIONS"
if [[ $NTM_SESSIONS -gt 10 ]]; then
    warn "Many NTM sessions - consider cleanup"
fi
echo

# RCH status
echo "8. RCH (Remote Compilation Helper)"
echo "   ─────────────────────────────────"
if [[ "$RCH_RUNNING" == "true" ]]; then
    pass "RCH daemon running - builds will be offloaded"
else
    warn "RCH daemon NOT running - all builds are local"
    echo "   Fix: rch daemon start"
fi
echo

# Top CPU consumers
echo "9. TOP CPU CONSUMERS"
echo "   ──────────────────"
ps aux --sort=-%cpu | head -6 | tail -5 | awk '{printf "   %5s %5s%% %s\n", $2, $3, $11}'
echo

# Recommendations
echo "10. RECOMMENDATIONS"
echo "    ────────────────"
RECOMMENDATIONS=0

if float_gt "$LOAD_RATIO" 1.5; then
    echo "    - Kill stuck tests: pkill -O 43200 -f 'bun test'"
    RECOMMENDATIONS=$((RECOMMENDATIONS + 1))
fi

if [[ $ZOMBIE_COUNT -gt 10 ]]; then
    echo "    - Investigate zombie parents"
    RECOMMENDATIONS=$((RECOMMENDATIONS + 1))
fi

if [[ "$RCH_RUNNING" != "true" ]]; then
    echo "    - Start RCH: rch daemon start"
    RECOMMENDATIONS=$((RECOMMENDATIONS + 1))
fi

if [[ $NTM_SESSIONS -gt 10 ]]; then
    echo "    - Clean stale tmux sessions"
    RECOMMENDATIONS=$((RECOMMENDATIONS + 1))
fi

if [[ $RECOMMENDATIONS -eq 0 ]]; then
    pass "System healthy - no immediate action needed"
fi

echo
echo "================================================================"
echo "                  DIAGNOSTIC COMPLETE"
echo "================================================================"
