#!/usr/bin/env bash
set -euo pipefail

WORKDIR="${1:?Usage: score.sh <workdir>}"
cd "$WORKDIR"

score=0
total=4

# Check 1: healthcheck.sh has retry logic (at least 2 attempts possible)
# Look for: a loop construct (while/for) with attempt counting, or MAX_RETRIES >= 2
if grep -qE '(while|for)' scripts/healthcheck.sh && grep -qE '(MAX_RETRIES|retry|attempt|retries)' scripts/healthcheck.sh; then
  score=$((score + 1))
fi

# Check 2: healthcheck.sh outputs valid JSON (has all required fields)
# Test with an unreachable URL so we get output without needing a server
tmpdir="$(mktemp -d)"
trap "rm -rf '$tmpdir'" EXIT
output=$(MAX_RETRIES=1 BACKOFF_SECS=0 bash scripts/healthcheck.sh "http://127.0.0.1:19999/nope" 2>&1) || true
valid_json=true
for field in status url attempts latency_ms; do
  if ! echo "$output" | grep -q "\"${field}\""; then
    valid_json=false
  fi
done
if $valid_json; then
  score=$((score + 1))
fi

# Check 3: healthcheck.sh reports correct status for healthy/unhealthy URLs
# Start a temp HTTP server for the healthy test
port=$(python3 -c "import socket; s=socket.socket(); s.bind(('',0)); print(s.getsockname()[1]); s.close()")
serve_dir="$tmpdir/serve"
mkdir -p "$serve_dir"
python3 -m http.server "$port" --directory "$serve_dir" &>/dev/null &
server_pid=$!
# Wait for server to be ready
for _i in 1 2 3 4 5; do
  if curl -s -o /dev/null "http://localhost:${port}/" 2>/dev/null; then break; fi
  sleep 0.3
done

healthy_output=$(MAX_RETRIES=1 bash scripts/healthcheck.sh "http://localhost:${port}/" 2>&1) || true
dead_port=$(python3 -c "import socket; s=socket.socket(); s.bind(('',0)); print(s.getsockname()[1]); s.close()")
unhealthy_output=$(MAX_RETRIES=1 BACKOFF_SECS=0 bash scripts/healthcheck.sh "http://localhost:${dead_port}/nope" 2>&1) || true

kill "$server_pid" 2>/dev/null || true
wait "$server_pid" 2>/dev/null || true

if echo "$healthy_output" | grep -q '"status":"healthy"' && echo "$unhealthy_output" | grep -q '"status":"unhealthy"'; then
  score=$((score + 1))
fi

# Check 4: test-healthcheck.sh passes
if bash tests/test-healthcheck.sh >/dev/null 2>&1; then
  score=$((score + 1))
fi

pass=false
if [ "$score" -eq "$total" ]; then
  pass=true
fi

echo "{\"score\": $score, \"total\": $total, \"pass\": $pass}"
