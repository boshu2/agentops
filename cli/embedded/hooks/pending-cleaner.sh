#!/usr/bin/env bash
# pending-cleaner.sh — Monitor stale flywheel backlog and auto-clear safely.
# Called during session start to prevent queue buildup.
# Exit 0 always — never block session start.
# practices: [sre, resilience-patterns]

# Kill switches
[ "${AGENTOPS_HOOKS_DISABLED:-}" = "1" ] && exit 0
[ "${AGENTOPS_PENDING_CLEANER_DISABLED:-}" = "1" ] && exit 0

ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
ROOT="$(cd "$ROOT" 2>/dev/null && pwd -P 2>/dev/null || printf '%s' "$ROOT")"
AO_DIR="$ROOT/.agents/ao"
PENDING_FILE="$AO_DIR/pending.jsonl"
LEGACY_PENDING_DIR="$AO_DIR/pending"
ARCHIVE_DIR="$AO_DIR/archive"
LOG_FILE="$AO_DIR/hook-errors.log"
STALE_SECONDS="${AGENTOPS_PENDING_STALE_SECONDS:-172800}"  # 2 days
ALERT_LINES="${AGENTOPS_PENDING_ALERT_LINES:-50}"

log_event() {
    mkdir -p "$AO_DIR" 2>/dev/null || return 0
    echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) pending-cleaner: $1" >> "$LOG_FILE" 2>/dev/null || true
}

mtime_epoch() {
    local file="$1"
    if stat -f %m "$file" >/dev/null 2>&1; then
        stat -f %m "$file" 2>/dev/null
    elif stat -c %Y "$file" >/dev/null 2>&1; then
        stat -c %Y "$file" 2>/dev/null
    else
        echo ""
    fi
}

monitor_pending_jsonl() {
    local now mtime age lines timestamp archive_name archive_path

    [ -f "$PENDING_FILE" ] || return 0
    [ -s "$PENDING_FILE" ] || return 0

    lines=$(wc -l < "$PENDING_FILE" 2>/dev/null | tr -d ' ' || echo "0")
    if [ "$lines" -ge "$ALERT_LINES" ] 2>/dev/null; then
        log_event "ALERT backlog pending.jsonl lines=$lines threshold=$ALERT_LINES"
    fi

    now=$(date +%s 2>/dev/null || echo "")
    mtime=$(mtime_epoch "$PENDING_FILE")
    if [ -z "$now" ] || [ -z "$mtime" ]; then
        log_event "WARN unable to compute stale age for pending.jsonl"
        return 0
    fi

    age=$((now - mtime))
    if [ "$age" -lt "$STALE_SECONDS" ] 2>/dev/null; then
        return 0
    fi

    log_event "ALERT stale pending.jsonl detected age_seconds=$age lines=$lines threshold=$STALE_SECONDS"

    mkdir -p "$ARCHIVE_DIR" 2>/dev/null || {
        log_event "ERROR unable to create archive dir: $ARCHIVE_DIR"
        return 0
    }

    timestamp=$(date -u +%Y%m%dT%H%M%SZ)
    archive_name="pending-${timestamp}.jsonl"
    archive_path="$ARCHIVE_DIR/$archive_name"

    if cp "$PENDING_FILE" "$archive_path" 2>/dev/null; then
        : > "$PENDING_FILE" 2>/dev/null || rm -f "$PENDING_FILE" 2>/dev/null
        log_event "AUTOCLEAR stale pending.jsonl archived=$archive_name"
    else
        log_event "ERROR failed to archive stale pending.jsonl"
    fi
}

archive_legacy_pending_files() {
    local stale_files timestamp file basename archive_name

    [ -d "$LEGACY_PENDING_DIR" ] || return 0
    stale_files=$(find "$LEGACY_PENDING_DIR" -name "*.jsonl" -mtime +2 2>/dev/null || true)
    [ -n "$stale_files" ] || return 0

    mkdir -p "$ARCHIVE_DIR" 2>/dev/null || {
        log_event "ERROR unable to create archive dir for legacy pending: $ARCHIVE_DIR"
        return 0
    }

    timestamp=$(date -u +%Y%m%dT%H%M%SZ)

    echo "$stale_files" | while IFS= read -r file; do
        [ -f "$file" ] || continue
        basename=$(basename "$file")
        archive_name="${timestamp}-${basename}"
        if cp "$file" "$ARCHIVE_DIR/$archive_name" 2>/dev/null; then
            rm -f "$file" 2>/dev/null
            log_event "archived legacy stale: $basename -> $archive_name"
        else
            log_event "ERROR failed to archive legacy stale file: $basename"
        fi
    done
}

monitor_pending_jsonl || true
archive_legacy_pending_files || true

exit 0
