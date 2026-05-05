#!/usr/bin/env bash
set -euo pipefail

# Service health checker with retry, backoff, and JSON output.
# Usage: healthcheck.sh [URL]
# Falls back to config/deploy.yaml health_url if no argument given.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG="${HEALTHCHECK_CONFIG:-${SCRIPT_DIR}/../config/deploy.yaml}"
MAX_RETRIES="${MAX_RETRIES:-3}"
BACKOFF_SECS="${BACKOFF_SECS:-2}"

URL="${1:-}"
if [[ -z "$URL" && -f "$CONFIG" ]]; then
    URL="$(grep "^health_url:" "$CONFIG" 2>/dev/null | awk -F': ' '{print $2}' | tr -d '\r')"
fi

if [[ -z "$URL" ]]; then
    echo '{"status":"unhealthy","url":"","attempts":0,"latency_ms":0,"error":"no URL provided"}' >&2
    exit 1
fi

# --- HTTP check (curl) or TCP fallback ---

check_http() {
    local url="$1"
    local start_ms end_ms code
    start_ms=$(date +%s%N)
    code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 "$url" 2>/dev/null) || code="000"
    end_ms=$(date +%s%N)
    LATENCY_MS=$(( (end_ms - start_ms) / 1000000 ))
    [[ "$code" == "200" ]]
}

check_tcp() {
    local host port
    host="$(echo "$1" | sed -E 's|https?://||;s|/.*||;s|:.*||')"
    port="$(echo "$1" | sed -E 's|https?://||;s|/.*||' | grep -oE ':[0-9]+' | tr -d ':')"
    port="${port:-80}"
    local start_ms end_ms
    start_ms=$(date +%s%N)
    (echo >/dev/tcp/"$host"/"$port") 2>/dev/null
    local rc=$?
    end_ms=$(date +%s%N)
    LATENCY_MS=$(( (end_ms - start_ms) / 1000000 ))
    return $rc
}

LATENCY_MS=0
attempt=0
healthy=false

while (( attempt < MAX_RETRIES )); do
    attempt=$((attempt + 1))
    if command -v curl &>/dev/null; then
        if check_http "$URL"; then
            healthy=true
            break
        fi
    else
        if check_tcp "$URL"; then
            healthy=true
            break
        fi
    fi
    if (( attempt < MAX_RETRIES )); then
        sleep "$BACKOFF_SECS"
    fi
done

if $healthy; then
    status="healthy"
    rc=0
else
    status="unhealthy"
    rc=1
fi

printf '{"status":"%s","url":"%s","attempts":%d,"latency_ms":%d}\n' \
    "$status" "$URL" "$attempt" "$LATENCY_MS"
exit $rc
