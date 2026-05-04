#!/usr/bin/env bash
set -euo pipefail

WORKDIR="${1:?Usage: setup.sh <workdir>}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
GOLDEN="$(cd "$SCRIPT_DIR/../../devops" && pwd)"

mkdir -p "$WORKDIR"
cp -r "$GOLDEN/." "$WORKDIR/"

# Remove the graceful empty-directory handling from rotate-logs.sh.
# The golden version uses find -print0 which naturally produces nothing on
# empty dirs, and the "No logs to rotate" report block exits cleanly.
# We break it by:
# 1. Replacing the safe find-based compress loop with ls piped to a while-read
#    that crashes when ls finds nothing (ls *.log fails under set -e in empty dirs)
# 2. Removing the zero-count early-exit report block

TARGET="$WORKDIR/scripts/rotate-logs.sh"

# Replace the find-based compress loop with an ls-based approach that crashes
# on empty directories (ls *.log exits non-zero when no .log files exist,
# which kills the script under set -euo pipefail)
sed -i '/^# --- Compress logs older than RETENTION_DAYS ---$/,/^done/c\
# --- Compress logs older than RETENTION_DAYS ---\
\
compressed=0\
ls "$LOG_DIR"/*.log | while read -r file; do\
    if [[ $(( $(date +%s) - $(stat -c %Y "$file") )) -gt $(( RETENTION_DAYS * 86400 )) ]]; then\
        gzip "$file"\
        compressed=$((compressed + 1))\
    fi\
done' "$TARGET"

# Remove the "No logs to rotate" early-exit block so empty dirs just fall through
# to the broken report section
sed -i '/^if \[\[ \$compressed -eq 0 && \$deleted -eq 0 \]\]/,/^fi$/d' "$TARGET"
