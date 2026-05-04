#!/usr/bin/env bash
set -euo pipefail

# Log rotation: compress old logs, delete ancient compressed logs.
# Usage: rotate-logs.sh <log_directory> [retention_days]

LOG_DIR="${1:-}"
RETENTION_DAYS="${2:-7}"

if [[ -z "$LOG_DIR" ]]; then
    echo "Usage: rotate-logs.sh <log_directory> [retention_days]" >&2
    exit 1
fi

if [[ ! -d "$LOG_DIR" ]]; then
    echo "ERROR: directory does not exist: $LOG_DIR" >&2
    exit 1
fi

DELETE_DAYS=$(( RETENTION_DAYS * 2 ))

# --- Compress logs older than RETENTION_DAYS ---

compressed=0
while IFS= read -r -d '' file; do
    gzip "$file"
    compressed=$((compressed + 1))
done < <(find "$LOG_DIR" -maxdepth 1 -type f -name '*.log' -mtime +"$RETENTION_DAYS" -print0 2>/dev/null)

# --- Delete compressed logs older than 2*RETENTION_DAYS ---

deleted=0
while IFS= read -r -d '' file; do
    rm -f "$file"
    deleted=$((deleted + 1))
done < <(find "$LOG_DIR" -maxdepth 1 -type f -name '*.log.gz' -mtime +"$DELETE_DAYS" -print0 2>/dev/null)

# --- Report ---

if [[ $compressed -eq 0 && $deleted -eq 0 ]]; then
    echo "No logs to rotate in $LOG_DIR (retention=${RETENTION_DAYS}d, delete=${DELETE_DAYS}d)"
    exit 0
fi

echo "Log rotation complete in $LOG_DIR:"
[[ $compressed -gt 0 ]] && echo "  Compressed: $compressed file(s) older than ${RETENTION_DAYS} days"
[[ $deleted -gt 0 ]]    && echo "  Deleted:    $deleted file(s) older than ${DELETE_DAYS} days"
exit 0
