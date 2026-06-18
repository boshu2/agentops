#!/usr/bin/env bash
# process-contention-sweep.sh — diagnose (don't kill) parasitic processes
#
# Usage: process-contention-sweep.sh [--include-kill-cmds]
#
# Lists processes that commonly hold cross-session locks:
#   - br create/close/update/sync (bead DB SQLite lock)
#   - cargo test/check/bench/build (registry lock + disk)
#   - rsync running >15 min (filesystem I/O)
#   - D-state (uninterruptible I/O) — can block SQLite filesystem-wide
#
# By default PRINTS ONLY. Pass --include-kill-cmds to emit suggested kill commands
# (still printed to stdout, NOT executed — per AGENTS.md RULE 1 never auto-kill).
#
# Per OC-031 (Sweep Cross-Session Zombies) + RECOVERY.md cross-session contention.

set -u
INCLUDE_KILL=0
[ "${1:-}" = "--include-kill-cmds" ] && INCLUDE_KILL=1

hr() { printf '── %s ──\n' "$*"; }

hr "BR processes (bead DB contention)"
pgrep -af 'br (create|close|update|sync|list)' 2>/dev/null | while read -r PID REST; do
  CWD=$(readlink "/proc/$PID/cwd" 2>/dev/null || echo "?")
  ETIME=$(ps -o etime= -p "$PID" 2>/dev/null | tr -d ' ')
  printf '  pid=%s etime=%s cwd=%s  cmd=%s\n' "$PID" "$ETIME" "$CWD" "$REST"
done

hr "Cargo/build processes (registry + target-dir contention)"
pgrep -af 'cargo (test|check|bench|build|run)' 2>/dev/null | while read -r PID REST; do
  CWD=$(readlink "/proc/$PID/cwd" 2>/dev/null || echo "?")
  ETIME=$(ps -o etime= -p "$PID" 2>/dev/null | tr -d ' ')
  printf '  pid=%s etime=%s cwd=%s  cmd=%s\n' "$PID" "$ETIME" "$CWD" "$REST"
done

hr "Long-running rsync (>15 min)"
ps -eo pid,etime,comm,args | awk '
  $3 == "rsync" && ( $2 ~ /-/ || $2 ~ /^[0-9]+:[0-9]+:[0-9]+$/ || ($2 ~ /^[0-9]+:[0-9]+$/ && substr($2,1,index($2,":")-1)+0 >= 15) ) {print "  "$0}
'

hr "D-state (uninterruptible I/O) processes"
ps -eo pid,stat,etime,comm,args | awk '$2 ~ /D/ {print "  "$0}'

hr "Disk delta hint"
df -h / | awk 'NR==2 {print "  /: used="$3" avail="$4" use%="$5}'

if [ "$INCLUDE_KILL" = 1 ]; then
  hr "SUGGESTED kill commands (PRINTED ONLY — review before running)"
  echo "  # Review each PID's cwd above. Only kill processes whose cwd is NOT in your live swarm."
  echo "  # Graceful first, SIGKILL only if process survives:"
  echo "  #   kill -15 <pid>; sleep 5; kill -0 <pid> 2>/dev/null && kill -9 <pid>"
  echo "  # D-state processes will NOT respond to kill -9; route around them."
fi
