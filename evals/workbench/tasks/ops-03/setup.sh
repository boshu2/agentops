#!/usr/bin/env bash
set -euo pipefail

WORKDIR="${1:?Usage: setup.sh <workdir>}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
GOLDEN="$(cd "$SCRIPT_DIR/../../devops" && pwd)"

mkdir -p "$WORKDIR"
cp -r "$GOLDEN/." "$WORKDIR/"

# Remove retry logic from healthcheck.sh — make it try exactly once
# and fail immediately on any error.
TARGET="$WORKDIR/scripts/healthcheck.sh"

# Replace the retry loop with a single-attempt check (no loop, no backoff)
sed -i '/^LATENCY_MS=0$/,/^done$/c\
LATENCY_MS=0\
attempt=1\
healthy=false\
if command -v curl \&>/dev/null; then\
    if check_http "$URL"; then\
        healthy=true\
    fi\
else\
    if check_tcp "$URL"; then\
        healthy=true\
    fi\
fi' "$TARGET"

# Also remove the MAX_RETRIES and BACKOFF_SECS config lines since they're unused now
sed -i '/^MAX_RETRIES=/d' "$TARGET"
sed -i '/^BACKOFF_SECS=/d' "$TARGET"
