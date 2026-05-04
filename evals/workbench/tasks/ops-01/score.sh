#!/usr/bin/env bash
set -euo pipefail

WORKDIR="${1:?Usage: score.sh <workdir>}"
cd "$WORKDIR"

score=0
total=4

# Check 1: deploy.sh has set -e or set -euo pipefail
if grep -qE '^\s*set\s+-[a-z]*e' scripts/deploy.sh; then
  score=$((score + 1))
fi

# Check 2: deploy.sh exits non-zero when given invalid config (missing required field)
tmpdir="$(mktemp -d)"
trap "rm -rf '$tmpdir'" EXIT
cat > "$tmpdir/bad.yaml" <<EOF
app_name: test-app
EOF
if ! DEPLOY_CONFIG="$tmpdir/bad.yaml" bash scripts/deploy.sh >/dev/null 2>&1; then
  score=$((score + 1))
fi

# Check 3: deploy.sh succeeds with valid config
cat > "$tmpdir/good.yaml" <<EOF
app_name: test-app
version: "1.0.0"
target: staging
log_dir: $tmpdir/logs
EOF
if DEPLOY_CONFIG="$tmpdir/good.yaml" bash scripts/deploy.sh >/dev/null 2>&1; then
  score=$((score + 1))
fi

# Check 4: test-deploy.sh passes
if bash tests/test-deploy.sh >/dev/null 2>&1; then
  score=$((score + 1))
fi

pass=false
if [ "$score" -eq "$total" ]; then
  pass=true
fi

echo "{\"score\": $score, \"total\": $total, \"pass\": $pass}"
